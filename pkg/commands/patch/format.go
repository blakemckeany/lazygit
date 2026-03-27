package patch

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/generics/set"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/samber/lo"
)

type patchPresenter struct {
	patch *Patch
	// if true, all following fields are ignored
	plain bool

	// line indices for tagged lines (e.g. lines added to a custom patch)
	incLineIndices *set.Set[int]

	// if true, show line number gutter
	showLineNumbers bool
}

// formats the patch as a plain string
func formatPlain(patch *Patch) string {
	presenter := &patchPresenter{
		patch:          patch,
		plain:          true,
		incLineIndices: set.New[int](),
	}
	return presenter.format()
}

func formatRangePlain(patch *Patch, startIdx int, endIdx int) string {
	lines := patch.Lines()[startIdx : endIdx+1]
	return strings.Join(
		lo.Map(lines, func(line *PatchLine, _ int) string {
			return line.Content + "\n"
		}),
		"",
	)
}

type FormatViewOpts struct {
	// line indices for tagged lines (e.g. lines added to a custom patch)
	IncLineIndices *set.Set[int]
	// if true, show line number gutter
	ShowLineNumbers bool
}

// formats the patch for rendering within a view, meaning it's coloured and
// highlights selected items
func formatView(patch *Patch, opts FormatViewOpts) string {
	includedLineIndices := opts.IncLineIndices
	if includedLineIndices == nil {
		includedLineIndices = set.New[int]()
	}
	presenter := &patchPresenter{
		patch:           patch,
		plain:           false,
		incLineIndices:  includedLineIndices,
		showLineNumbers: opts.ShowLineNumbers,
	}
	return presenter.format()
}

// Returns the max width of the gutter for rendering
func GutterWidth(patch *Patch, showLineNumbers bool) int {
	if !showLineNumbers {
		return 0
	}
	digitWidth := max(len(fmt.Sprintf("%d", patch.MaxLineNumber())), 3)

	return 2*digitWidth + 6
}

func (self *patchPresenter) format() string {
	// if we have no changes in our patch (i.e. no additions or deletions) then
	// the patch is effectively empty and we can return an empty string
	if !self.patch.ContainsChanges() {
		return ""
	}

	digitWidth := 0
	if self.showLineNumbers && !self.plain {
		digitWidth = len(fmt.Sprintf("%d", self.patch.MaxLineNumber()))
		if digitWidth < 3 {
			digitWidth = 3
		}
	}

	stringBuilder := &strings.Builder{}
	lineIdx := 0
	appendLine := func(line string) {
		_, _ = stringBuilder.WriteString(line + "\n")

		lineIdx++
	}

	for _, line := range self.patch.header {
		gutter := self.formatGutter(-1, -1, digitWidth)
		// always passing false for 'included' here because header lines are not part of the patch
		appendLine(gutter + self.formatLineAux(line, theme.DefaultTextColor.SetBold(), false))
	}

	for _, hunk := range self.patch.hunks {
		gutter := self.formatGutter(-1, -1, digitWidth)
		appendLine(
			gutter +
				self.formatLineAux(
					hunk.formatHeaderStart(),
					style.FgCyan,
					false,
				) +
				// we're splitting the line into two parts: the diff header and the context
				// We explicitly pass 'included' as false for both because these are not part
				// of the actual patch
				self.formatLineAux(
					hunk.headerContext,
					theme.DefaultTextColor,
					false,
				),
		)

		oldLineNum := hunk.oldStart
		newLineNum := hunk.newStart

		for _, line := range hunk.bodyLines {
			var gutter string
			switch line.Kind {
			case CONTEXT:
				gutter = self.formatGutter(oldLineNum, newLineNum, digitWidth)
				oldLineNum++
				newLineNum++
			case ADDITION:
				gutter = self.formatGutter(-1, newLineNum, digitWidth)
				newLineNum++
			case DELETION:
				gutter = self.formatGutter(oldLineNum, -1, digitWidth)
				oldLineNum++
			default:
				gutter = self.formatGutter(-1, -1, digitWidth)
			}

			lineStyle := self.patchLineStyle(line)
			if line.IsChange() {
				appendLine(gutter + self.formatLine(line.Content, lineStyle, lineIdx))
			} else {
				appendLine(gutter + self.formatLineAux(line.Content, lineStyle, false))
			}
		}
	}

	return stringBuilder.String()
}

func (self *patchPresenter) formatGutter(oldNum, newNum int, digitWidth int) string {
	if digitWidth == 0 || self.plain {
		return ""
	}

	gutterStyle := style.FgCyan

	newStr := fmt.Sprintf("%*s", digitWidth, "")
	if newNum >= 0 {
		newStr = fmt.Sprintf("%*d", digitWidth, newNum)
	}

	return gutterStyle.Sprintf("%s │ ", newStr)
}

func (self *patchPresenter) patchLineStyle(patchLine *PatchLine) style.TextStyle {
	switch patchLine.Kind {
	case ADDITION:
		return style.FgGreen
	case DELETION:
		return style.FgRed
	default:
		return theme.DefaultTextColor
	}
}

func (self *patchPresenter) formatLine(str string, textStyle style.TextStyle, index int) string {
	included := self.incLineIndices.Includes(index)

	return self.formatLineAux(str, textStyle, included)
}

// 'selected' means you've got it highlighted with your cursor
// 'included' means the line has been included in the patch (only applicable when
// building a patch)
func (self *patchPresenter) formatLineAux(str string, textStyle style.TextStyle, included bool) string {
	if self.plain {
		return str
	}

	firstCharStyle := textStyle
	if included {
		firstCharStyle = firstCharStyle.MergeStyle(style.BgGreen)
	}

	if len(str) < 2 {
		return firstCharStyle.Sprint(str)
	}

	return firstCharStyle.Sprint(str[:1]) + textStyle.Sprint(str[1:])
}
