package valfs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/goccy/go-yaml"
	yamlcomment "github.com/zijiren233/yaml-comment"
)

type ValPackage struct {
	Val    *valgo.ExtendedVal
	client *common.Client
}

type ValFrontmatterLinks struct {
	Website  string  `yaml:"valtown" lc:"🔒"`
	Module   string  `yaml:"esmModule" lc:"🔒"`
	Endpoint *string `yaml:"deployment,omitempty" lc:"🔒"`
	Email    *string `yaml:"email,omitempty" lc:"🔒"`
}

type ValFrontmatter struct {
	Id      string              `yaml:"id" lc:"🔒"`
	Version int32               `yaml:"version" lc:"🔒 (reopen file to see change)"`
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
	if len(contents) < 4 {
		return nil, nil, errors.New("No code found")
	}
	frontmatterFindIndex := frontmatterRe.FindStringIndex(contents)
	if frontmatterFindIndex == nil {
		return nil, nil, errors.New("Invalid frontmatter format")
	}
	frontmatterEndIndex := frontmatterFindIndex[1]

	var offset int
	if contents[frontmatterEndIndex+3] == '\n' {
		offset = 4
	} else {
		offset = 3
	}
	codeSection := contents[frontmatterEndIndex+offset : len(contents)-1]

	// Deserialize the frontmatter
	meta = &ValFrontmatter{}
	err = yaml.Unmarshal([]byte(frontmatterMatch), meta)
	if err != nil {
		return nil, nil, err
	}

	return &codeSection, meta, nil
}

// Create a new val package from a val
func NewValPackage(client *common.Client, val *valgo.ExtendedVal) ValPackage {
	return ValPackage{Val: val, client: client}
}

// Get just the metadata text
func (v *ValPackage) GetFrontmatterText() (string, error) {
	link := v.Val.GetLinks()

	if v.client.Config.StaticMeta {
		if strings.Contains(link.Module, "?") {
			link.Module = strings.Split(link.Module, "?")[0]
		}
	}

	frontmatterValLinks := ValFrontmatterLinks{
		Website:  v.Val.Url,
		Module:   link.Module,
		Endpoint: link.Endpoint,
	}

	if v.Val.Type == "email" {
		if v.Val.GetAuthor().Username.Get() == nil {
			return "", errors.New("Author username is nil")
		}

		emailAddress := fmt.Sprintf("%s.%s@valtown.email", *v.Val.GetAuthor().Username.Get(), v.Val.Name)
		frontmatterValLinks.Email = &emailAddress
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

	return "/*---\n" + string(frontmatterYAML) + "---*/\n\n", nil
}

// Convert the val contained in the ValPackage to a package with metadata at the
// top and the code of the val underneath
func (v *ValPackage) ToText() (*string, error) {
	frontmatter, err := v.GetFrontmatterText()
	if err != nil {
		return nil, err
	}

	combined := frontmatter + v.Val.GetCode()

	if v.client.Config.ExecutableVals {
		combined = AffixShebang(combined)
	}

	return &combined, nil
}

func (v *ValPackage) Len() (int, error) {
	contents, err := v.ToText()
	if err != nil {
		return 0, err
	}

	return len(*contents), nil
}

// Set the contents of a val package. Updates underlying val by deconstructing
// the contents into frontmatter and code.
func (v *ValPackage) UpdateVal(contents string) error {
	code, frontmatter, err := DeconstructVal(contents)
	if err != nil {
		common.Logger.Error("Error deconstructing val", err)
		return err
	}

	// Update the underlying val
	v.Val.Privacy = frontmatter.Privacy
	v.Val.SetReadme(frontmatter.Readme)
	v.Val.SetCode(*code)

	// Success
	return nil
}
