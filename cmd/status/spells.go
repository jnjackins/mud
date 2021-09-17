package main

type spellKind int

const (
	damage spellKind = iota
	area
	debuff
	buff
	healing
	utility
)

var spellKinds = map[string]spellKind{
	"rend":                 damage,
	"acid burn":            damage,
	"energy drain":         damage,
	"pain":                 damage,
	"magic blast":          damage,
	"malevolent tentacles": area,
	"acid mist":            area,
	"flesh shield":         buff,
	"silence":              debuff,
	"hold monster":         debuff,

	"soul leech": healing,
	"nightfall":  buff,
	"bless":      buff,

	"regenerate":    healing,
	"cure critical": healing,
	"cure serious":  healing,
	"ghostskin":     buff,
	"heal boost":    buff,
}
