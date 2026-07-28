package browser

import (
	"fmt"
	"html"
	"strings"
	"unicode"
)

type RenderOptions struct {
	ProgramID string
	MountID   string
}

func RenderHTML[Msg any](root Node[Msg], options RenderOptions) (string, error) {
	var out strings.Builder
	if err := renderNode(&out, root, options, nil); err != nil {
		return "", err
	}
	return out.String(), nil
}

func renderNode[Msg any](out *strings.Builder, node Node[Msg], options RenderOptions, path []int) error {
	match node.Kind {
	case NodeText():
		out.WriteString(html.EscapeString(node.Text))
	case NodeFragment():
		out.WriteString("<cadence-fragment")
		writeAttribute(out, "style", "display:contents")
		if options.ProgramID != "" {
			writeAttribute(out, "data-cadence-program", options.ProgramID)
		}
		writeAttribute(out, "data-cadence-node", pathName(path))
		if node.Key != "" {
			writeAttribute(out, "data-cadence-key", node.Key)
		}
		out.WriteByte('>')
		for index, child := range node.Children {
			if err := renderNode(out, child, options, childPath(path, index)); err != nil {
				return err
			}
		}
		out.WriteString("</cadence-fragment>")
	case NodeElement():
		if !validName(node.Tag) {
			return fmt.Errorf("quicken/web/browser: invalid element name %q", node.Tag)
		}
		out.WriteByte('<')
		out.WriteString(node.Tag)
		if len(path) == 0 && options.MountID != "" {
			writeAttribute(out, "id", options.MountID)
		}
		if options.ProgramID != "" {
			writeAttribute(out, "data-cadence-program", options.ProgramID)
		}
		writeAttribute(out, "data-cadence-node", pathName(path))
		if node.Key != "" {
			writeAttribute(out, "data-cadence-key", node.Key)
		}
		for _, attribute := range node.Attributes {
			if !validName(attribute.Name) {
				return fmt.Errorf("quicken/web/browser: invalid attribute name %q", attribute.Name)
			}
			match attribute.Kind {
			case AttributeString(), AttributeProperty():
				writeAttribute(out, attribute.Name, attribute.Value)
			case AttributeBoolean():
				if attribute.Value == "true" {
					out.WriteByte(' ')
					out.WriteString(attribute.Name)
				}
			}
		}
		for _, event := range node.Events {
			writeAttribute(out, "data-cadence-on-"+EventKindName(event.Kind), event.ID)
		}
		out.WriteByte('>')
		for index, child := range node.Children {
			if err := renderNode(out, child, options, childPath(path, index)); err != nil {
				return err
			}
		}
		out.WriteString("</")
		out.WriteString(node.Tag)
		out.WriteByte('>')
	}
	return nil
}

func writeAttribute(out *strings.Builder, name, value string) {
	out.WriteByte(' ')
	out.WriteString(name)
	out.WriteString(`="`)
	out.WriteString(html.EscapeString(value))
	out.WriteByte('"')
}

func validName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':' {
			if index == 0 && unicode.IsDigit(r) {
				return false
			}
			continue
		}
		return false
	}
	return true
}

func pathName(path []int) string {
	if len(path) == 0 {
		return "root"
	}
	parts := make([]string, len(path))
	for index, value := range path {
		parts[index] = fmt.Sprint(value)
	}
	return strings.Join(parts, ".")
}
