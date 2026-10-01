package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type node struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Content []node `json:"content"`
	Attrs   struct {
		URL       string `json:"url"`
		Text      string `json:"text"`
		ShortName string `json:"shortName"`
		Order     int    `json:"order"`
		Language  string `json:"language"`
	} `json:"attrs"`
	Marks []struct {
		Type  string `json:"type"`
		Attrs struct {
			Href string `json:"href"`
		} `json:"attrs"`
	} `json:"marks"`
}

func descriptionText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	var root node
	if err := json.Unmarshal(raw, &root); err != nil || root.Type != "doc" {
		return "", fmt.Errorf("expected an Atlassian Document Format document")
	}
	return strings.TrimSpace(renderNode(root)), nil
}

func renderNode(n node) string {
	switch n.Type {
	case "bulletList", "orderedList":
		var list strings.Builder
		order := n.Attrs.Order
		if order < 1 {
			order = 1
		}
		for i, item := range n.Content {
			prefix := "- "
			if n.Type == "orderedList" {
				prefix = strconv.Itoa(order+i) + ". "
			}
			itemText := strings.TrimSpace(renderNode(item))
			list.WriteString(prefix + strings.ReplaceAll(itemText, "\n", "\n  ") + "\n")
		}
		return list.String() + "\n"
	case "tableRow":
		cells := make([]string, 0, len(n.Content))
		for _, cell := range n.Content {
			cells = append(cells, strings.TrimSpace(renderNode(cell)))
		}
		return strings.Join(cells, " | ") + "\n"
	}
	var children strings.Builder
	for _, child := range n.Content {
		children.WriteString(renderNode(child))
	}
	text := children.String()
	switch n.Type {
	case "text":
		text = n.Text
		for _, mark := range n.Marks {
			if mark.Type == "link" && mark.Attrs.Href != "" {
				text += " (" + mark.Attrs.Href + ")"
			}
		}
		return text
	case "hardBreak":
		return "\n"
	case "paragraph", "heading", "blockquote":
		return text + "\n\n"
	case "codeBlock":
		return "```" + n.Attrs.Language + "\n" + strings.TrimRight(text, "\n") + "\n```\n\n"
	case "table":
		return text + "\n"
	case "rule":
		return "---\n\n"
	case "inlineCard", "blockCard":
		return n.Attrs.URL + text
	case "mention":
		return n.Attrs.Text + text
	case "emoji":
		return n.Attrs.ShortName + text
	default:
		// Unknown containers still contribute their descendant text.
		return n.Text + text
	}
}
