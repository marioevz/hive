package helper

import (
	"fmt"
	"strings"

	"github.com/lithammer/dedent"
)

// Description struct to hold the description of a test structured in different categories
type Description struct {
	Main        string
	Subsections map[string][]string
}

const (
	CategoryTestnetConfiguration         = "Testnet Configuration"
	CategoryVerificationsExecutionClient = "Verifications (Execution Client)"
	CategoryVerificationsConsensusClient = "Verifications (Consensus Client)"
)

// NewDescription creates a new instance of Description
func NewDescription(main string) *Description {
	return &Description{
		Main:        main,
		Subsections: make(map[string][]string),
	}
}

// Copy creates a copy of the description
func (d *Description) Copy() *Description {
	subsections := make(map[string][]string)
	for category, items := range d.Subsections {
		subsections[category] = items[:]
	}
	return &Description{
		Main:        d.Main,
		Subsections: subsections,
	}
}

// Add method to add an item to a category
func (d *Description) Add(category, item string) {
	// Check if the category already exists
	_, exists := d.Subsections[category]
	if !exists {
		// Create a new category if it doesn't exist
		d.Subsections[category] = []string{}
	}
	// Append the item to the category
	d.Subsections[category] = append(d.Subsections[category], item)
}

func (d *Description) Format() string {
	// Create a string builder
	sb := strings.Builder{}
	// Add the main description
	sb.WriteString(dedent.Dedent(d.Main))
	// Iterate over the categories
	for category, Subsections := range d.Subsections {
		// Add the category to the string builder
		sb.WriteString(fmt.Sprintf("\n\n#### %s\n\n", category))
		// Iterate over the Subsections
		for _, item := range Subsections {
			// Add the item to the string builder
			sb.WriteString(dedent.Dedent(item))
		}
	}
	return sb.String()
}
