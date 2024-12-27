package fuse

import (
	"errors"
	"github.com/404wolf/valgo"
	"strings"

	"gopkg.in/yaml.v2"
)

// Combines a BasicVal and content into a single string with YAML frontmatter
func PackVal(val valgo.BasicVal, content string) (string, error) {
	frontmatterVal := struct {
		Name    string `yaml:"name"`
		Id      string `yaml:"id"`
		Version int32  `yaml:"version"`
		Public  bool   `yaml:"public"`
		Privacy string `yaml:"privacy"`
		Type    string `yaml:"type"`
	}{
		Name:    val.Name,
		Id:      val.Id,
		Version: val.Version,
		Public:  val.Public,
		Privacy: val.Privacy,
		Type:    val.Type,
	}

	frontmatterYAML, err := yaml.Marshal(frontmatterVal)
	if err != nil {
		return "", err
	}

	combined := "/*---\n" + string(frontmatterYAML) + "---*/\n\n" + content

	return combined, nil
}

// Separates a combined string into BasicVal and content
func UnpackVal(combined string) (valgo.BasicVal, string, error) {
	var val valgo.BasicVal
	var content string

	start := strings.Index(combined, "/*---")
	end := strings.Index(combined, "---*/")

	if start == -1 || end == -1 {
		return val, content, errors.New("frontmatter not found")
	}

	frontmatterYAML := combined[start+5 : end]
	frontmatterYAML = strings.TrimSpace(frontmatterYAML)

	err := yaml.Unmarshal([]byte(frontmatterYAML), &val)
	if err != nil {
		return val, content, err
	}

	content = strings.TrimSpace(combined[end+5:])

	return val, content, nil
}
