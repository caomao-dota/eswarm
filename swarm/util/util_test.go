package util

import (
	"os"
	"testing"
)

func TestCipher(t *testing.T) {

	rawData := Defs{[]string{"enode://0e4aa108144d8678f82306985bf71ab65004bc248f2f239b9fc2085b7daaea8ee317d3178664b04e7104e6a7840a5c61e2f89cf9603ef11e4b5006b5c91dd46d@110.185.107.116:30400?nt=9"}, "https://service.cdscfund.org/apis/v1"}
	//rawData := Defs{[]string{"enode://d8578bfef5b8e447d9e3ca674597b317a8ceb103da129a978d93eed61888e4cfaab19794e3c2e274a88de3ce238ae70fd5556f0365f323abbea2011f605861d6@172.16.1.11:30396"}, "http://172.16.1.10:4000/apis/v1"}

	result := CiphData(rawData)

	// open output file
	fo, err := os.Create("output.txt")
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	if _, err := fo.WriteString(result); err != nil {
		panic(err)
	}

	def, err := DecipherData(result)
	if err == nil {
		t.Log("result:", def)
	} else {
		t.Error("error in cipher/decipher", err)
	}
}
