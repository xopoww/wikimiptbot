package main

import (
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/anaskhan96/soup"
)

func pageUrl(name string) string {
	return "http://wikimipt.org/wiki/" + name
}

var badCharError = errors.New("bad symbols in query")
const searchUrl = "http://wikimipt.org/index.php?title=Категория:Преподаватели_по_алфавиту&from="
var nonCyrillicRE = regexp.MustCompile("[^а-я ]")
func search(query string) ([]string, error){
	query = strings.ReplaceAll(strings.ToLower(query), "ё", "е")
	if nonCyrillicRE.FindStringIndex(query) != nil {
		return []string{}, badCharError
	}
	queryRune := []rune(query)

	resp, err := soup.Get(searchUrl+string(queryRune[0]))
	if err != nil {
		log.Fatal(err)
		return []string{}, err
	}
	doc := soup.HTMLParse(resp)
	as := doc.FindStrict("div", "class", "mw-category-group").FindAll("a")

	var results []string
	for _, a := range as {
		name := a.Attrs()["title"]
		if strings.HasPrefix(strings.ToLower(name), query) {
			results = append(results, name)
		}
	}

	return results, nil
}

type mark struct {
	value float64
	votes int64
}
type teacherProfile struct {
	name string
	photo string
	stats [5]mark
	desc string
}

var (
	tagHeads = map[string]string{
		"i":"_",
		"b":"*",
		"p":"",
		"ul":"",
		"li":" • ",
		"br":"\n",
	}
	tagTails = map[string]string{
		"i":"_",
		"b":"*",
		"p":"\n",
		"ul":"",
		"li":"\n",
		"br":"",
	}
)
var (
	mustEscapeRE = regexp.MustCompile("["+regexp.QuoteMeta("_*[]()~`>#+=|{}.!-")+"]")
	nonWhitespaceRE = regexp.MustCompile("\\S")
)
func escapeMarkdownV2(s string) string {
	return mustEscapeRE.ReplaceAllStringFunc(s, func(ss string) string {return "\\"+ss})
}
func parseTag(tag soup.Root, safe bool) (string, error) {
	if tag.Pointer.Type == html.TextNode {
		if nonWhitespaceRE.FindStringIndex(tag.NodeValue) == nil {
			return "", nil
		}
		return escapeMarkdownV2(tag.NodeValue), nil
	}
	if tag.Pointer.Type == html.CommentNode {
		return "", nil
	}

	result := ""
	if tag.NodeValue == "a" {
		url := strings.ReplaceAll(tag.Attrs()["href"], ")", "\\)")
		if url[0:1] == "/" {
			url = "wikimipt.org" + url
		}
		result += "["
		for _, child := range tag.Children() {
			subres, err := parseTag(child, safe)
			if err != nil {
				return "", err
			}
			result += subres
		}
		result += fmt.Sprintf("](%s)", url)
	} else if head, ok := tagHeads[tag.NodeValue]; ok || safe {
		result += head
		for _, child := range tag.Children() {
			subres, err := parseTag(child, safe)
			if err != nil {
				return "", err
			}
			result += subres
		}
		result += tagTails[tag.NodeValue]
	} else {
		return "", errors.New("not supported tag: "+tag.NodeValue)
	}
	return result, nil
}

func getProfile (name string) (teacherProfile, error) {
	link := pageUrl(name)
	resp, err := soup.Get(link)
	if err != nil {
		return teacherProfile{}, err
	}

	var result teacherProfile
	result.name = name

	doc := soup.HTMLParse(resp)
	table := doc.FindStrict("table", "class", "wikitable card")
	if img := table.FindAll("tr")[1].Find("img"); img.Pointer != nil {
		result.photo = "wikimipt.org" + img.Attrs()["src"]
	}
	statsRows := table.Find("table").FindAll("tr")
	for index, row := range statsRows {
		var (
			value float64
			votes int64
		)
		text := row.FindStrict("span", "class", "starrating-avg").FullText()
		if text == "( нет голосов )" {
			value = 0.
			votes = 0
		} else {
			splitText := strings.Split(text, " ")
			var err error
			if value, err = strconv.ParseFloat(splitText[0], 32); err != nil {
				return teacherProfile{}, err
			}
			if votes, err = strconv.ParseInt(splitText[2], 10, 0); err != nil {
				return teacherProfile{}, err
			}
		}
		result.stats[index] = mark{value, votes}
	}

	for tag := table.FindNextSibling(); tag.NodeValue != "div"; tag = tag.FindNextSibling() {
		parsed, err := parseTag(tag, true)
		if err != nil {
			return teacherProfile{}, err
		}
		result.desc += parsed
	}

	return result, nil
}