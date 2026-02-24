package bazel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

type BazelBinary struct {
	path      string
	workspace string
	delay     time.Duration
}

func NewBazel(workspace string) (Queries, error) {
	path, err := exec.LookPath("bazel")
	if err != nil {
		return nil, err
	}

	return &BazelBinary{
		path:      path,
		workspace: workspace,
	}, nil
}

type Label string

func (l Label) String() string {
	return string(l)
}

func (l Label) ToWorkspacePath(workspaceDir string) string {
	parts := strings.SplitN(string(l), ":", 2)

	return path.Join(workspaceDir, strings.TrimPrefix(parts[0], "//"))
}

func (l Label) TargetName() string {
	parts := strings.SplitN(string(l), ":", 2)

	if len(parts) == 1 {
		_, name := path.Split(strings.TrimPrefix(string(l), "//"))
		return name
	} else {
		return parts[1]
	}
}

type MaxRankResult struct {
	// Ordered list as returned by MaxRank
	Ranking map[int][]Label
}

type Queries interface {
	QueryMaxRank(ctx context.Context, args ...string) (MaxRankResult, error)
	QueryLocation(ctx context.Context, args ...string) (LocationResult, error)
	QueryXml(ctx context.Context, args ...string) (XmlResult, error)
	QueryFiles(ctx context.Context, args ...string) ([]string, error)
	QueryStarlarkJson(ctx context.Context, target any, args ...string) error

	Build(ctx context.Context, label Label) error
	Run(ctx context.Context, label Label, env map[string]string, args ...string) error
	// Workspace returns the path to the base of the Bazel workspace
	Workspace() string
}

type parser func(context.Context, io.Reader) error

type cmdConfig func(*exec.Cmd) error

func noOpConfigure(_ *exec.Cmd) error {
	return nil
}

func (b *BazelBinary) Workspace() string {
	return b.workspace
}

func (b *BazelBinary) QueryMaxRank(ctx context.Context, args ...string) (MaxRankResult, error) {
	ranks := MaxRankResult{
		Ranking: map[int][]Label{},
	}
	maxRank := func(ctx context.Context, reader io.Reader) error {
		bufrd := bufio.NewReader(reader)

		for {
			line, lineErr := bufrd.ReadString('\n')
			if errors.Is(lineErr, io.EOF) {
				break
			} else if lineErr != nil {
				return lineErr
			}

			parts := strings.SplitN(strings.TrimSpace(line), " ", 2)

			rank, intErr := strconv.Atoi(parts[0])
			if intErr != nil {
				return intErr
			}
			label := Label(parts[1])

			cur, ok := ranks.Ranking[rank]
			if !ok {
				cur = make([]Label, 0)
			}
			ranks.Ranking[rank] = append(cur, label)
		}

		return nil
	}

	fullArgs := append([]string{"query", "--output=maxrank"}, args...)

	err := b.exec(ctx, maxRank, noOpConfigure, fullArgs...)
	return ranks, err
}

type Location struct {
	Path  string
	Label Label
}

type LocationResult struct {
	Locations []Location
}

func LabelToPath(label Label) string {
	parts := strings.SplitN(string(label), ":", 2)

	return strings.TrimPrefix(parts[0], "//")
}

func (b *BazelBinary) QueryLocation(ctx context.Context, args ...string) (LocationResult, error) {
	locs := LocationResult{
		Locations: make([]Location, 0),
	}
	location := func(ctx context.Context, reader io.Reader) error {
		bufrd := bufio.NewReader(reader)

		for {
			line, lineErr := bufrd.ReadString('\n')
			if errors.Is(lineErr, io.EOF) {
				break
			} else if lineErr != nil {
				return lineErr
			}

			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)

			seg := strings.Split(parts[1], " ")

			var loc Location
			loc.Path = parts[0]
			loc.Label = Label(seg[len(seg)-1])

			locs.Locations = append(locs.Locations, loc)
		}

		return nil
	}

	fullArgs := append([]string{"query", "--output=location"}, args...)

	err := b.exec(ctx, location, noOpConfigure, fullArgs...)
	return locs, err
}

func (b *BazelBinary) QueryFiles(ctx context.Context, args ...string) ([]string, error) {
	outs := make([]string, 0)

	location := func(ctx context.Context, reader io.Reader) error {
		bufrd := bufio.NewReader(reader)

		for {
			line, lineErr := bufrd.ReadString('\n')
			if errors.Is(lineErr, io.EOF) {
				break
			} else if lineErr != nil {
				return lineErr
			}

			outs = append(outs, line)
		}

		return nil
	}

	fullArgs := append([]string{"cquery", "--output=files"}, args...)

	err := b.exec(ctx, location, noOpConfigure, fullArgs...)
	return outs, err
}

func (b *BazelBinary) QueryStarlarkJson(ctx context.Context, target any, args ...string) error {
	jsonParse := func(ctx context.Context, reader io.Reader) error {
		decoder := json.NewDecoder(reader)
		return decoder.Decode(target)
	}

	fullArgs := append([]string{"cquery", "--output=starlark"}, args...)

	err := b.exec(ctx, jsonParse, noOpConfigure, fullArgs...)
	return err
}

func (b *BazelBinary) Run(ctx context.Context, label Label, env map[string]string, args ...string) error {
	run := func(ctx context.Context, reader io.Reader) error {
		_, err := io.Copy(os.Stdout, reader)

		return err
	}

	copyEnv := func(cmd *exec.Cmd) error {
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		for _, e := range os.Environ() {
			cmd.Env = append(cmd.Env, e)
		}

		return nil
	}

	fullArgs := append([]string{"run", string(label), "--"}, args...)

	return b.exec(ctx, run, copyEnv, fullArgs...)
}

func (b *BazelBinary) Build(ctx context.Context, label Label) error {
	run := func(ctx context.Context, reader io.Reader) error {
		_, err := io.Copy(os.Stdout, reader)
		return err
	}

	fullArgs := append([]string{"build"}, string(label))

	err := b.exec(ctx, run, noOpConfigure, fullArgs...)

	return err
}

func (b *BazelBinary) exec(ctx context.Context, parser parser, config cmdConfig, args ...string) error {
	cmd := exec.CommandContext(ctx, b.path, args...)
	cmd.Dir = b.workspace

	configErr := config(cmd)
	if configErr != nil {
		return configErr
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil
	}

	var stderrBuffer bytes.Buffer
	cmd.Stderr = &stderrBuffer

	startErr := cmd.Start()
	if startErr != nil {
		return startErr
	}

	praseErr := parser(ctx, stdout)
	cmdErr := cmd.Wait()

	var copyErr error
	if cmdErr != nil {
		_, copyErr = io.Copy(os.Stderr, &stderrBuffer)
	}

	return errors.Join(cmdErr, praseErr, copyErr)
}

func (b *BazelBinary) QueryXml(ctx context.Context, args ...string) (XmlResult, error) {
	var result XmlResult

	xmlQuery := func(ctx context.Context, reader io.Reader) error {
		bytes, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		xmlData := []byte(strings.Replace(string(bytes), "version=\"1.1\"", "", 1))

		xmlErr := xml.Unmarshal(xmlData, &result)

		return xmlErr
	}

	fullArgs := append([]string{"query", "--output=xml"}, args...)

	err := b.exec(ctx, xmlQuery, noOpConfigure, fullArgs...)
	if err != nil {
		return result, err
	}

	return result, nil
}

type XmlResult struct {
	XMLName xml.Name `xml:"query"`
	Text    string   `xml:",chardata"`
	Version string   `xml:"version,attr"`
	Rule    []struct {
		Text     string `xml:",chardata"`
		Class    string `xml:"class,attr"`
		Location string `xml:"location,attr"`
		Name     string `xml:"name,attr"`
		String   []struct {
			Text  string `xml:",chardata"`
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"string"`
		List struct {
			Text  string `xml:",chardata"`
			Name  string `xml:"name,attr"`
			Label struct {
				Text  string `xml:",chardata"`
				Value string `xml:"value,attr"`
			} `xml:"label"`
		} `xml:"list"`
		Label struct {
			Text  string `xml:",chardata"`
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"label"`
		RuleInput struct {
			Text string `xml:",chardata"`
			Name string `xml:"name,attr"`
		} `xml:"rule-input"`
	} `xml:"rule"`
}
