package main

import (
	"strings"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const modifier = 0.10

func init() {
	algo.Init("default")
}

func fuzzyScore(matchable []string, text string) float64 {
	textLength := len(text)

	if textLength == 0 {
		return 1
	}

	multiplier := 0

	highest := 0.0

	for k, t := range matchable {
		if t == "" {
			continue
		}

		chars := util.ToChars([]byte(t))
		res, _ := algo.FuzzyMatchV2(false, true, true, &chars, []rune(strings.ToLower(text)), true, nil)

		if res.Start == -1 {
			continue
		}

		score := float64(res.Score - res.Start)

		if score == 0 {
			continue
		}

		if score > highest {
			multiplier = k
			highest = score
		}
	}

	if highest == 0 {
		return 0
	}

	m := (1 - modifier*float64(multiplier))

	if m < 0.7 {
		m = 0.7
	}

	return highest * m
}
