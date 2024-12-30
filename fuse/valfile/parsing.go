package fuse

import (
	"errors"
	"log"
	"regexp"

	"github.com/404wolf/valgo"
	"github.com/goccy/go-yaml"
	yamlcomment "github.com/zijiren233/yaml-comment"
)

type ValPackage struct {
	Val *valgo.ExtendedVal
}

type ValFrontmatterLinks struct {
	Website  string  `yaml:"valtown" lc:"🔒"`
	Module   string  `yaml:"esmModule" lc:"🔒"`
	Endpoint *string `yaml:"deployment,omitempty" lc:"🔒"`
}

type ValFrontmatter struct {
	Id      string              `yaml:"id" lc:"🔒"`
	Version int32               `yaml:"version" lc:"🔒"`
	Privacy string              `yaml:"privacy" lc:"(public|private|unlisted)"`
	Links   ValFrontmatterLinks `yaml:"links"`
	Readme  string              `yaml:"readme"`
}

// Break apart a val into its metadata that the user can edit, and the raw code
// contents
func DeconstructVal(contents string) (
	code *string,
	meta *ValFrontmatter,
	err error,
) {
	// Extract the frontmatter
	frontmatterRe := regexp.MustCompile(`(?s)/?---\n(.*?)\n---/?`)
	frontmatterMatches := frontmatterRe.FindStringSubmatch(contents)
	if len(frontmatterMatches) == 0 {
		return nil, nil, errors.New("No frontmatter found")
	}
	frontmatterMatch := frontmatterMatches[0]

	// Extract the code
	frontmatterEndIndex := frontmatterRe.FindStringIndex(contents)[1]
	codeSection := contents[frontmatterEndIndex+4:]

	// Deserialize the frontmatter
	meta = &ValFrontmatter{}
	err = yaml.Unmarshal([]byte(frontmatterMatch), meta)
	if err != nil {
		return nil, nil, err
	}

	return &codeSection, meta, nil
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
		Website:  v.Val.Url,
		Module:   link.Module,
		Endpoint: link.Endpoint,
	}
	frontmatterVal := ValFrontmatter{
		Id:      v.Val.Id,
		Version: v.Val.Version,
		Privacy: v.Val.Privacy,
		Links:   frontmatterValLinks,
		Readme:  v.Val.GetReadme(),
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
	code, frontmatter, err := DeconstructVal(contents)
	if err != nil {
		log.Println(err)
		return err
	}

	// Update the underlying val
	v.Val.Privacy = frontmatter.Privacy
	v.Val.SetReadme(frontmatter.Readme)
	v.Val.SetCode(*code)

	// Success
	return nil
}
