package valfs

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	common "github.com/404wolf/valfs/common"
	"github.com/goccy/go-yaml"
	yamlcomment "github.com/zijiren233/yaml-comment"
)

// ValPackage represents a complete val package with metadata and configuration
type ValPackage struct {
	Val Val

	StaticMeta     bool
	ExecutableVals bool
}

// valPackageFrontmatterLinks contains all the external links and references
type valPackageFrontmatterLinks struct {
	Valtown  string  `yaml:"valtown" lc:"🔒"`
	Module   string  `yaml:"esmModule" lc:"🔒"`
	Endpoint *string `yaml:"deployment,omitempty" lc:"🔒"`
	Email    *string `yaml:"email,omitempty" lc:"🔒"`
}

// valPackageFrontmatter represents the metadata section of a val package
type valPackageFrontmatter struct {
	Id      string                     `yaml:"id" lc:"🔒"`
	Version int32                      `yaml:"version" lc:"🔒 (reopen file to see change)"`
	Privacy string                     `yaml:"privacy" lc:"(public|private|unlisted)"`
	Links   valPackageFrontmatterLinks `yaml:"links"`
	ReadMe  string                     `yaml:"readme"`
}

// NewValPackage creates a new val package from a val
func NewValPackage(val Val, staticMeta bool, executableVals bool) ValPackage {
	return ValPackage{Val: val}
}

// ToText converts the val to a package with metadata and code
func (v *ValPackage) ToText() (*string, error) {
	frontmatter, err := v.getFrontmatterText()
	if err != nil {
		return nil, err
	}

	combined := frontmatter + v.Val.GetCode()

	if v.ExecutableVals {
		combined = AffixShebang(combined)
	}

	return &combined, nil
}

// Len returns the length of the code segment of the val package
func (v *ValPackage) Len() (int, error) {
	contents, err := v.ToText()
	if err != nil {
		return 0, err
	}

	return len(*contents), nil
}

// UpdateVal sets the contents of a val package and updates the underlying val
func (v *ValPackage) UpdateVal(contents string) error {
	code, frontmatter, err := deconstructVal(contents)
	if err != nil {
		common.Logger.Error("Error deconstructing val", err)
		return err
	}

	// Update the underlying val
	v.Val.SetPrivacy(frontmatter.Privacy)
	v.Val.SetReadme(frontmatter.ReadMe)
	v.Val.SetCode(*code)

	return nil
}

// deconstructVal breaks apart a val into its metadata and code contents
func deconstructVal(contents string) (
	code *string,
	meta *valPackageFrontmatter,
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
	meta = &valPackageFrontmatter{}
	err = yaml.Unmarshal([]byte(frontmatterMatch), meta)
	if err != nil {
		return nil, nil, err
	}

	return &codeSection, meta, nil
}

// getFrontmatterText returns just the metadata text
func (v *ValPackage) getFrontmatterText() (string, error) {
	moduleLink := v.Val.GetModuleLink()

	if v.StaticMeta {
		if strings.Contains(moduleLink, "?") {
			moduleLink = strings.Split(moduleLink, "?")[0]
		}
	}

	endpointLink := v.Val.GetEndpointLink()
	frontmatterValLinks := valPackageFrontmatterLinks{
		Valtown:  getWebsiteLink(v.Val.GetAuthorName(), v.Val.GetName()),
		Module:   moduleLink,
		Endpoint: getEndpointLinkPtr(endpointLink),
	}

	if v.Val.GetValType() == "email" {
		emailAddress := fmt.Sprintf("%s.%s@valtown.email", v.Val.GetAuthorName(), v.Val.GetName())
		frontmatterValLinks.Email = &emailAddress
	}

	frontmatterVal := valPackageFrontmatter{
		Id:      v.Val.GetId(),
		Version: v.Val.GetVersion(),
		Privacy: v.Val.GetPrivacy(),
		Links:   frontmatterValLinks,
		ReadMe:  v.Val.GetReadme(),
	}

	frontmatterYAML, err := yamlcomment.Marshal(frontmatterVal)
	if err != nil {
		return "", err
	}

	return "/*---\n" + string(frontmatterYAML) + "---*/\n\n", nil
}

// getWebsiteLink constructs the val.town website URL for a val
func getWebsiteLink(authorUsername, valName string) string {
	return fmt.Sprintf("https://www.val.town/v/%s/%s", authorUsername, valName)
}

// getEndpointLinkPtr converts empty endpoint links to nil, otherwise returns pointer
func getEndpointLinkPtr(endpointLink string) *string {
	if endpointLink == "" {
		return nil
	}
	return &endpointLink
}
