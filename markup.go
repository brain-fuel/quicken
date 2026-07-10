package quicken

import (
	"errors"
	"strings"
)

type markupKind int

const (
	kindLiteral markupKind = iota
	kindHead
	kindLazy
	kindLive
)

type markupSeg struct {
	kind markupKind
	text string
}

const markupOpen = "<!--quicken "
const markupClose = "-->"

// parseMarkup splits s into literal runs and quicken markers. A marker is the
// HTML comment <!--quicken <directive>--> where directive is "head",
// "lazy <id>", or "live <id>". A sentinel with no closing --> is left as
// literal text. A malformed directive or an invalid id is an error.
func parseMarkup(s string) ([]markupSeg, error) {
	var segs []markupSeg
	for {
		i := strings.Index(s, markupOpen)
		if i < 0 {
			if s != "" {
				segs = append(segs, markupSeg{kindLiteral, s})
			}
			return segs, nil
		}
		rest := s[i+len(markupOpen):]
		j := strings.Index(rest, markupClose)
		if j < 0 {
			// No close: the whole remainder, including the sentinel, is literal.
			segs = append(segs, markupSeg{kindLiteral, s})
			return segs, nil
		}
		if i > 0 {
			segs = append(segs, markupSeg{kindLiteral, s[:i]})
		}
		seg, err := parseDirective(rest[:j])
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
		s = rest[j+len(markupClose):]
	}
}

func parseDirective(d string) (markupSeg, error) {
	fields := strings.Fields(d)
	if len(fields) == 0 {
		return markupSeg{}, errors.New("quicken: empty marker directive")
	}
	switch fields[0] {
	case "head":
		if len(fields) != 1 {
			return markupSeg{}, errors.New("quicken: head marker takes no argument")
		}
		return markupSeg{kindHead, ""}, nil
	case "lazy", "live":
		if len(fields) != 2 {
			return markupSeg{}, errors.New("quicken: " + fields[0] + " marker needs exactly one id")
		}
		if !validID(fields[1]) {
			return markupSeg{}, errors.New("quicken: invalid region id " + fields[1])
		}
		if fields[0] == "lazy" {
			return markupSeg{kindLazy, fields[1]}, nil
		}
		return markupSeg{kindLive, fields[1]}, nil
	default:
		return markupSeg{}, errors.New("quicken: unknown marker directive " + fields[0])
	}
}
