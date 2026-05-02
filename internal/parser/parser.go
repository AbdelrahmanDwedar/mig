package parser

type Parser interface {
	Parse(content string) (up, down string, err error)
}
