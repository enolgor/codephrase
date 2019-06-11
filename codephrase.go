package codephrase

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/icza/bitio"
)

/*
func main() {
	pattern, err := CompilePattern("color animal verb adverb")
	if err != nil {
		panic(err)
	}

	code, sizes, phrase := randomPhrase(pattern)
	codebytes, size, err := join(code, sizes)
	if err != nil {
		panic(err)
	}
	fmt.Println(phrase)
	fmt.Println(code, sizes)
	fmt.Printf("%X %d\n", codebytes, size)

	code2, sizes2, err := getCodeFromPhrase(phrase, pattern)
	if err != nil {
		panic(err)
	}
	fmt.Println(code2, sizes2)
	codebytes2, size2, err := join(code, sizes)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%X %d\n", codebytes2, size2)
}*/

type Pattern interface {
	Size() int
	GetRandomPhrase() ([]string, []byte)
	Parse([]string) ([]byte, error)
	GetPhrase([]byte) ([]string, error)
}

type tokenType uint8

type patternToken struct {
	tokenType
	bits uint8
}

const animal = tokenType(0x00)
const verb = tokenType(0x01)
const color = tokenType(0x02)
const adverb = tokenType(0x03)
const adjective = tokenType(0x04)

type tokenTypeDef struct {
	maxBits uint8
	table   map[string]uint64
	array   []string
}

var str2Token = map[string]tokenType{
	"animal":    animal,
	"verb":      verb,
	"color":     color,
	"adverb":    adverb,
	"adjective": adjective,
}

var tokenTypeDefMap = map[tokenType]tokenTypeDef{
	animal:    tokenTypeDef{8, animalsMap, animals},
	verb:      tokenTypeDef{10, verbsMap, verbs},
	color:     tokenTypeDef{6, colorsMap, colors},
	adverb:    tokenTypeDef{11, adverbsMap, adverbs},
	adjective: tokenTypeDef{10, adjectivesMap, adjectives},
}

type patternImpl struct {
	tokens []patternToken
	size   int
}

func (pattern *patternImpl) Size() int {
	return pattern.size
}

func (pattern *patternImpl) GetRandomPhrase() ([]string, []byte) {
	code, sizes, phrase := randomPhrase(pattern)
	codebytes, _, err := join(code, sizes)
	if err != nil {
		panic(err)
	}
	return phrase, codebytes
}

func (pattern *patternImpl) Parse(phrase []string) ([]byte, error) {
	code, sizes, err := getCodeFromPhrase(phrase, pattern)
	if err != nil {
		return nil, err
	}
	codebytes, _, err := join(code, sizes)
	if err != nil {
		return nil, err
	}
	return codebytes, nil
}

func (pattern *patternImpl) GetPhrase(codebytes []byte) ([]string, error) {
	sizes := make([]uint8, len(pattern.tokens))
	for idx, token := range pattern.tokens {
		sizes[idx] = token.bits
	}
	code, _, err := getCode(codebytes, sizes)
	if err != nil {
		return nil, err
	}
	phrase := getPhraseFromCode(code, pattern)
	return phrase, nil
}

var tokenRegex = regexp.MustCompile("^(\\w+)+(?:\\((\\d+)\\))*$")

func MustCompilePattern(s string) Pattern {
	pattern, err := CompilePattern(s)
	if err != nil {
		panic(err)
	}
	return pattern
}

func CompilePattern(s string) (Pattern, error) {
	tokens := strings.Fields(s)
	pattern := &patternImpl{tokens: make([]patternToken, len(tokens)), size: 0}
	var tokenType tokenType
	var bits int
	var ok bool
	var err error
	for idx, token := range tokens {
		submatch := tokenRegex.FindStringSubmatch(token)
		if len(submatch) == 0 {
			return nil, fmt.Errorf("Pattern error. Should be: \"type1(bits) type2(bits) ...\"")
		}
		if tokenType, ok = str2Token[submatch[1]]; !ok {
			return nil, fmt.Errorf("TokenType not found")
		}
		if bits, err = strconv.Atoi(submatch[2]); err != nil {
			bits = int(tokenTypeDefMap[tokenType].maxBits)
		}
		pattern.size += bits
		pattern.tokens[idx] = patternToken{tokenType, uint8(bits)}
	}
	return pattern, nil
}

func getPhraseFromCode(code []uint64, pattern *patternImpl) []string {
	phrase := make([]string, len(code))
	for idx, token := range pattern.tokens {
		phrase[idx] = tokenTypeDefMap[token.tokenType].array[code[idx]]
	}
	return phrase
}

func getCodeFromPhrase(phrase []string, pattern *patternImpl) ([]uint64, []uint8, error) {
	code := make([]uint64, len(phrase))
	sizes := make([]uint8, len(pattern.tokens))
	var ok bool
	for idx, token := range pattern.tokens {
		code[idx], ok = tokenTypeDefMap[token.tokenType].table[phrase[idx]]
		sizes[idx] = token.bits
		if !ok {
			return nil, nil, fmt.Errorf("String %s not found in table", phrase[idx])
		}
	}
	return code, sizes, nil
}

func getCode(codebytes []byte, sizes []uint8) ([]uint64, int, error) {
	code := make([]uint64, len(sizes))
	r := bitio.NewReader(bytes.NewBuffer(codebytes))
	totalSize := 0
	var err error
	for idx, size := range sizes {
		if code[idx], err = r.ReadBits(size); err != nil {
			return nil, 0, err
		}
		totalSize += int(size)
	}
	return code, totalSize, nil
}

func join(code []uint64, sizes []uint8) ([]byte, int, error) {
	b := &bytes.Buffer{}
	w := bitio.NewWriter(b)
	size := 0
	var err error
	for idx, c := range code {
		if err = w.WriteBits(c, sizes[idx]); err != nil {
			return nil, 0, err
		}
		size += int(sizes[idx])
	}
	if err = w.Close(); err != nil {
		return nil, 0, err
	}
	return b.Bytes(), size, nil
}

func randomPhrase(pattern *patternImpl) ([]uint64, []uint8, []string) {
	phrase := make([]string, len(pattern.tokens))
	code := make([]uint64, len(pattern.tokens))
	sizes := make([]uint8, len(pattern.tokens))
	for idx, token := range pattern.tokens {
		code[idx], phrase[idx], sizes[idx] = getRandomItem(&token)
	}
	return code, sizes, phrase
}

func getRandomItem(t *patternToken) (uint64, string, uint8) {
	tokenTypeDef := tokenTypeDefMap[t.tokenType]
	bits := t.bits
	if t.bits == 0 || t.bits > tokenTypeDef.maxBits {
		bits = tokenTypeDef.maxBits
	}
	r := randBits(bits)
	return r, tokenTypeDef.array[r], bits
}

func randBits(bits uint8) uint64 {
	bytes := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		panic(err)
	}
	var mask uint64
	var i uint8
	for i = 0; i < bits; i++ {
		mask = mask << 1
		mask++
	}
	return binary.LittleEndian.Uint64(bytes) & mask
}
