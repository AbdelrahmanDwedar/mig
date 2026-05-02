package parser

import (
	"bufio"
	"strings"
)

type SQLParser struct{}

func (p *SQLParser) Parse(content string) (up, down string, err error) {
	var upBuf, downBuf strings.Builder
	var currentSection string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "+migrate Up") {
			currentSection = "up"
		} else if strings.Contains(line, "+migrate Down") {
			currentSection = "down"
		} else {
			if currentSection == "up" {
				upBuf.WriteString(line + "\n")
			} else if currentSection == "down" {
				downBuf.WriteString(line + "\n")
			}
		}
	}
	return strings.TrimSpace(upBuf.String()), strings.TrimSpace(downBuf.String()), nil
}
