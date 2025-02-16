package gotextfsm

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"sort"
	"strings"
)

const (
	LineSaturation  = 40
	LineLightness   = 60
	MatchSaturation = 100
	MatchLightness  = 30
)

const HTMLTemplateText = `
<!DOCTYPE html>
<html>
<head>
    <meta charset='UTF-8'>
    <title>TextFSM Visual Debugger</title>
    <style type='text/css'>
        body {
            font-family: Arial, Helvetica, sans-serif;
            background-color: hsl(40, 1%, 25%);
            margin: 0;
            padding: 0;
        }
        h4 {
            font-family: Arial, Helvetica, sans-serif;
            color: white;
            margin-top: 0;
        }
        .regex {
            background-color: silver;
            border: 2px solid black;
            display: none;
            border-radius: 5px;
            padding: 0 10px;
            position: absolute;
            margin-left: 10px;
            z-index: 100;
        }
        .cli-title {
            padding-top: 100px;
        }
        .states {
            position: fixed;
            background-color: dimgray;
            width: 100%;
            padding: 10px;
            margin-top: 0;
            box-shadow: 0 3px 8px #000000;
        }
        /* State styles */
        {{range .StateStyles}}
        .{{.Name}} {
            background-color: hsl({{.Hue}}, 40%, 60%);
            border-radius: 5px;
            padding: 0 10px;
            color: black;
        }
        {{end}}
        
        /* Match styles */
        {{range .MatchStyles}}
        .{{.Class}} {
            background-color: hsl({{.Hue}}, 100%, 30%);
            border-radius: 5px;
            font-weight: bold;
            color: white;
            padding: 0 5px;
        }
        .{{.Class}}:hover + .regex {
            display: inline;
        }
        {{end}}

        pre {
            white-space: pre-wrap;
            word-wrap: break-word;
            color: white;
        }
    </style>
</head>
<body>
    <header class='states'>
        <h4>States:</h4>
        {{range .StateStyles}}
        <button class='{{.Name}}'>{{.Name}}</button>
        {{end}}
    </header>
    <h4 class='cli-title'>CLI Text:</h4>
    <pre>{{.ProcessedText | unescapeHTML}}</pre>
</body>
</html>`

// StateStyle represents the styling for a state
type StateStyle struct {
	Name string
	Hue  int
}

// MatchStyle represents the styling for a match
type MatchStyle struct {
	Class string
	Hue   int
}

// IndexPair represents start and end indices of a match
type IndexPair struct {
	Start int
	End   int
	Value string
}

// VisualDebugger handles the creation of the debug visualization
type VisualDebugger struct {
	fsm           *TextFSM
	cliText       string
	stateColors   map[string]int
	stateStyles   []StateStyle
	matchStyles   []MatchStyle
	processedText string
}

// NewVisualDebugger creates a new VisualDebugger instance
func NewVisualDebugger(fsm *TextFSM, cliText string) *VisualDebugger {
	return &VisualDebugger{
		fsm:         fsm,
		cliText:     cliText,
		stateColors: make(map[string]int),
	}
}

// buildStateColors assigns colors to states using a color wheel approach
func (vd *VisualDebugger) buildStateColors() {
	counter := 1
	stateCount := len(vd.fsm.States)
	hueStep := 360 / (stateCount + 1) // Ensure even distribution around the color wheel

	for stateName := range vd.fsm.States {
		hue := (hueStep * counter) % 360
		vd.stateColors[stateName] = hue
		vd.stateStyles = append(vd.stateStyles, StateStyle{
			Name: stateName,
			Hue:  hue,
		})
		counter++
	}
}

// mergeIndexPairs merges overlapping index pairs
func (vd *VisualDebugger) mergeIndexPairs(pairs []IndexPair) []IndexPair {
	if len(pairs) <= 1 {
		return pairs
	}

	// Sort pairs by start index
	// Note: Implementation needed for sorting

	var merged []IndexPair
	current := pairs[0]

	for i := 1; i < len(pairs); i++ {
		if pairs[i].Start <= current.End {
			// Merge overlapping pairs
			current.End = int(math.Max(float64(current.End), float64(pairs[i].End)))
			current.Value = fmt.Sprintf("%s, %s", current.Value, pairs[i].Value)
		} else {
			merged = append(merged, current)
			current = pairs[i]
		}
	}
	merged = append(merged, current)

	return merged
}

func (vd *VisualDebugger) processLine(line string, state string, lineNum int, matches []IndexPair) string {
	if len(matches) == 0 {
		return template.HTMLEscapeString(line) + "\n"
	}

	// Sort matches by start position to ensure proper order
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	var result strings.Builder
	lastEnd := 0

	for i, match := range matches {
		// Verify the match indices are valid
		if match.Start < 0 || match.End < 0 || match.Start >= len(line) || match.End > len(line) {
			fmt.Printf("warning: invalid match indices: start=%d, end=%d, lineLen=%d\n",
				match.Start, match.End, len(line))
			continue
		}

		// Add text before match
		if match.Start > lastEnd {
			result.WriteString(template.HTMLEscapeString(line[lastEnd:match.Start]))
		}

		// Create match class
		matchClass := fmt.Sprintf("%s-match-%d-%d", state, lineNum, i)
		vd.matchStyles = append(vd.matchStyles, MatchStyle{
			Class: matchClass,
			Hue:   vd.stateColors[state],
		})

		// Add matched text with highlighting
		matchText := template.HTMLEscapeString(line[match.Start:match.End])
		result.WriteString(fmt.Sprintf("<span class='%s'>%s</span>", matchClass, matchText))

		// Add regex tooltip
		if value, exists := vd.fsm.Values[match.Value]; exists {
			regexText := template.HTMLEscapeString(fmt.Sprintf("%s >> %s", value.Regex, match.Value))
			result.WriteString(fmt.Sprintf("<span class='regex'>%s</span>", regexText))
		}

		lastEnd = match.End
	}

	// Add remaining text
	if lastEnd < len(line) {
		result.WriteString(template.HTMLEscapeString(line[lastEnd:]))
	}
	result.WriteString("\n")

	return result.String()
}

func (vd *VisualDebugger) GenerateHTML() (string, error) {
	vd.buildStateColors()

	lines := strings.Split(vd.cliText, "\n")
	var processedLines []string

	for i, line := range lines {
		var currentState string
		var matches []IndexPair

		if i < len(vd.fsm.parseHistory) {
			historyEntry := vd.fsm.parseHistory[i]
			currentState = historyEntry.StateName
			matches = historyEntry.MatchIndices
		}

		processedLine := vd.processLine(line, currentState, i, matches)
		processedLines = append(processedLines, processedLine)
	}

	vd.processedText = strings.Join(processedLines, "")

	// Create template with the unescapeHTML function
	funcMap := template.FuncMap{
		"unescapeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl, err := template.New("debug").Funcs(funcMap).Parse(HTMLTemplateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	data := struct {
		StateStyles     []StateStyle
		MatchStyles     []MatchStyle
		ProcessedText   string
		LineSaturation  int
		LineLightness   int
		MatchSaturation int
		MatchLightness  int
	}{
		StateStyles:     vd.stateStyles,
		MatchStyles:     vd.matchStyles,
		ProcessedText:   vd.processedText,
		LineSaturation:  LineSaturation,
		LineLightness:   LineLightness,
		MatchSaturation: MatchSaturation,
		MatchLightness:  MatchLightness,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	return buf.String(), nil
}
