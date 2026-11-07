package aiorders

import (
	"regexp"
	"sort"
	"strings"
)

var presentationWord = regexp.MustCompile("[A-Za-z][A-Za-z0-9_]*")
var presentationLabels = func() map[string]string {
	labels := map[string]string{}
	kinds := make([]string, 0, len(Definitions.Items))
	for kind := range Definitions.Items {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		item := Definitions.Items[kind]
		labels[strings.ToLower(kind)] = item.Label
		for _, field := range item.Fields {
			for code, label := range field.ValueLabels {
				key := strings.ToLower(code)
				if _, exists := labels[key]; !exists {
					labels[key] = label
				}
			}
		}
	}
	return labels
}()

// Only explanatory text goes through this formatter. Structured order values stay intact.
func DisplayText(text string) string {
	return presentationWord.ReplaceAllStringFunc(text, func(word string) string {
		if label := presentationLabels[strings.ToLower(word)]; label != "" {
			return label
		}
		return word
	})
}
