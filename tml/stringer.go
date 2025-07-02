package tml

import "strings"

func (r *TerritoryResult) String() string {
	var sb strings.Builder
	sb.WriteString("Territory: ")
	sb.WriteString(r.Territory.Name)

	sb.WriteString("\n")

	if !r.Success {
		sb.WriteString("Cannot display territory")
		return sb.String()
	}

	sb.WriteString("ID: ")
	sb.WriteString(r.Territory.ID)
	sb.WriteString("\n")

	sb.WriteString("Guild: ")
	sb.WriteString(r.Territory.Guild.Name)
	sb.WriteString(" [")
	sb.WriteString(r.Territory.Guild.Tag)
	sb.WriteString("]\n")

	return sb.String()
}