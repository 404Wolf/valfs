package fuse

import (
	"errors"
	"regexp"

	"github.com/404wolf/valgo"
	"github.com/goccy/go-yaml"
	yamlcomment "github.com/zijiren233/yaml-comment"
)

type ValPackage struct {
	Val *valgo.ExtendedVal
}

type ValFrontmatterLinks struct {
	Self     string  `yaml:"valtown"`
	Versions string  `yaml:"history"`
	Module   string  `yaml:"esmModule"`
	Endpoint *string `yaml:"deployment" lc:"only for HTTP vals"`
}

type ValFrontmatter struct {
	Version int32               `yaml:"version" lc:"don't change"`
	Privacy string              `yaml:"privacy" lc:"(public|private|unlisted)"`
	Links   ValFrontmatterLinks `yaml:"links"`
}

func (v *ValPackage) GetContents() (string, error) {
	link := v.Val.GetLinks()

	frontmatterValLinks := ValFrontmatterLinks{
		Self:     link.Self,
		Versions: link.Versions,
		Module:   link.Module,
		Endpoint: link.Endpoint,
	}
	frontmatterVal := ValFrontmatter{
		Version: v.Val.Version,
		Privacy: v.Val.Privacy,
		Links:   frontmatterValLinks,
	}

	frontmatterYAML, err := yamlcomment.Marshal(frontmatterVal)
	if err != nil {
		return "", err
	}

	combined := "/*---\n" + string(frontmatterYAML) + "---*/\n\n" + v.Val.GetCode()
	finalized := AffixShebang(combined) // add a shebang so val can be executed

	return finalized, nil
}

func (v *ValPackage) GetContentsLength() (int, error) {
	contents, err := v.GetContents()
	if err != nil {
		return 0, err
	}

	return len(contents), nil
}

// Set the contents of a val package. Updates underlying val by deconstructing
// the contents into frontmatter and code.
func (v *ValPackage) SetContents(contents string) error {
	// Extract the frontmatter
	re := regexp.MustCompile(`(?s)/*---\n(.*?)\n---*/`)
	matches := re.FindStringSubmatch(contents)
	if len(matches) == 0 {
		return errors.New("No frontmatter found")
	}
	match := matches[0]

	// Deserialize the frontmatter
	frontmatter := ValFrontmatter{}
	err := yaml.Unmarshal([]byte(match), &frontmatter)
	if err != nil {
		return err
	}

	// Update the underlying val
	v.Val.Privacy = frontmatter.Privacy

	// Success
	return nil
}
