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

// extractShebang returns the shebang line if present and the remaining content
func extractShebang(content string) (shebang string, remaining string) {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		if len(lines) > 1 {
			return lines[0], strings.TrimSpace(lines[1])
		}
		return lines[0], ""
	}
	return "", content
}

// extractMetadata finds and extracts the YAML metadata block from the content
func extractMetadata(content string) (metadata string, remaining string, err error) {
	// Try comment-wrapped format first: /*---...---*/
	commentWrappedRegex := regexp.MustCompile(`(?s)/\*---\n(.*?)\n---\*/`)
	if match := commentWrappedRegex.FindStringSubmatchIndex(content); match != nil {
		metadataBlock := content[match[2]:match[3]]
		beforeMetadata := content[:match[0]]
		afterMetadata := content[match[1]:]
		return metadataBlock, beforeMetadata + afterMetadata, nil
	}

	// Try plain format: ---...---
	plainRegex := regexp.MustCompile(`(?s)^---\n(.*?)\n---`)
	if match := plainRegex.FindStringSubmatchIndex(content); match != nil {
		metadataBlock := content[match[2]:match[3]]
		remaining := content[match[1]:]
		return metadataBlock, remaining, nil
	}

	return "", "", errors.New("no metadata block found")
}

// DeconstructVal breaks apart a val into its metadata and code contents
func DeconstructVal(contents string) (code *string, meta *ValFrontmatter, err error) {
	if len(contents) == 0 {
		return nil, nil, errors.New("empty contents")
	}

	// Extract shebang if present
	shebang, remainingContent := extractShebang(contents)

	// Extract metadata
	metadataStr, codeContent, err := extractMetadata(remainingContent)
	if err != nil {
		return nil, nil, err
	}

	// Parse metadata
	meta = &ValFrontmatter{}
	if err := yaml.Unmarshal([]byte(metadataStr), meta); err != nil {
		return nil, nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Clean up code content and handle shebang
	codeContent = strings.TrimSpace(codeContent)
	// Remove any additional shebangs from the code content
	if strings.HasPrefix(codeContent, "#!") {
		_, codeContent = extractShebang(codeContent)
	}

	// Add back the original shebang if it existed
	if shebang != "" {
		codeContent = shebang + "\n" + codeContent
	}

	return &codeContent, meta, nil
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

	// Handle email vals
	if v.Val.Type == "email" {
		if v.Val.GetAuthor().Username.Get() == nil {
			return "", errors.New("Author username is nil")
		}
		emailAddress := fmt.Sprintf("%s.%s@valtown.email", *v.Val.GetAuthor().Username.Get(), v.Val.Name)
		frontmatterValLinks.Email = &emailAddress
	}

	// Handle HTTP vals
	if v.Val.Type == "http" && link.Endpoint != nil {
		frontmatterValLinks.Endpoint = link.Endpoint
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

	code := v.Val.GetCode()

	// Remove any existing shebang from the code
	if strings.HasPrefix(code, "#!") {
		_, code = extractShebang(code)
	}

	combined := frontmatter + code

	// Add shebang if needed
	if v.client.Config.ExecutableVals {
		// Only add shebang if it's not already present
		if !strings.HasPrefix(combined, "#!") {
			combined = AffixShebang(combined)
		}
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

// UpdateVal sets the contents of a val package
func (v *ValPackage) UpdateVal(contents string) error {
	if v == nil {
		return errors.New("ValPackage is nil")
	}
	if v.Val == nil {
		return errors.New("Underlying Val is nil")
	}
	if len(contents) == 0 {
		return errors.New("Contents is empty")
	}

	code, frontmatter, err := DeconstructVal(contents)
	if err != nil {
		common.Logger.Error("Error deconstructing val", err)
		return err
	}

	if code == nil {
		return errors.New("Extracted code is nil")
	}
	if frontmatter == nil {
		return errors.New("Extracted frontmatter is nil")
	}

	// Validate privacy value
	validPrivacy := map[string]bool{
		"public":   true,
		"private":  true,
		"unlisted": true,
	}
	if !validPrivacy[frontmatter.Privacy] {
		return fmt.Errorf("Invalid privacy value: %s", frontmatter.Privacy)
	}

	v.Val.Privacy = frontmatter.Privacy
	v.Val.SetReadme(frontmatter.Readme)
	v.Val.SetCode(*code)

	return nil
}

// LooksLikeMetadata returns true if the string appears to contain metadata
func LooksLikeMetadata(contents string) bool {
	if len(contents) == 0 {
		return false
	}

	_, _, err := extractMetadata(contents)
	return err == nil
}
