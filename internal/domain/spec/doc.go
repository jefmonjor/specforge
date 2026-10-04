// Package spec models a SpecForge specification: a Markdown document whose
// acceptance criteria are written in Gherkin.
//
// The package is pure: it works on strings and never touches the
// filesystem. Parsing is delegated to the official Cucumber Gherkin parser,
// so every Gherkin dialect, And/But steps, Background, Rule and Scenario
// Outline with Examples behave exactly as in Cucumber.
package spec
