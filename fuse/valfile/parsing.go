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
	Self     string  `yaml:"valtown"lc:"🔒"`
	Versions string  `yaml:"history" lc:"🔒"`
	Module   string  `yaml:"esmModule" lc:"🔒"`
	Endpoint *string `yaml:"deployment" lc:"🔒 (only for HTTP vals)"`
}

type ValFrontmatter struct {
	Id      string              `yaml:"id" lc:"🔒"`
	Version int32               `yaml:"version" lc:"🔒"`
	Privacy string              `yaml:"privacy" lc:"(public|private|unlisted)"`
	Links   ValFrontmatterLinks `yaml:"links"`
}

// Break apart a val into its metadata that the user can edit, and the raw code
// contents
func deconstruct(contents string) (*ValFrontmatter, error) {
	// Extract the frontmatter
	re := regexp.MustCompile(`(?s)/*---\n(.*?)\n---*/`)
	matches := re.FindStringSubmatch(contents)
	if len(matches) == 0 {
		return nil, errors.New("No frontmatter found")
	}
	match := matches[0]

	// Deserialize the frontmatter
	frontmatter := ValFrontmatter{}
	err := yaml.Unmarshal([]byte(match), &frontmatter)
	if err != nil {
		return nil, err
	}

	return &frontmatter, nil
}

// Create a new val package from a val
func NewValPackage(val *valgo.ExtendedVal) ValPackage {
	return ValPackage{Val: val}
}

// Convert the val contained in the ValPackage to a package with metadata at the
// top and the code of the val underneath
func (v *ValPackage) ToText() (string, error) {
	link := v.Val.GetLinks()

	frontmatterValLinks := ValFrontmatterLinks{
		Self:     link.Self,
		Versions: link.Versions,
		Module:   link.Module,
		Endpoint: link.Endpoint,
	}
	frontmatterVal := ValFrontmatter{
		Id:      v.Val.Id,
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

func (v *ValPackage) Len() (int, error) {
	contents, err := v.ToText()
	if err != nil {
		return 0, err
	}

	return len(contents), nil
}

// Set the contents of a val package. Updates underlying val by deconstructing
// the contents into frontmatter and code.
func (v *ValPackage) UpdateVal(contents string) error {
	frontmatter, err := deconstruct(contents)
	if err != nil {
		return err
	}

	// Update the underlying val
	v.Val.Privacy = frontmatter.Privacy

	// Success
	return nil
}
