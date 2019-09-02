package util

import (
	"os"
	"testing"
)

func TestCipher(t *testing.T) {

	rawData := Defs{[]string{"enode://8f84bcad710281f20a41e0faff94ae38044a764ec3182fde705ddb9ff5b166684a87bc6e53e227a5a735b6f5fb4e9d97867e20c66ebe069b87694151326bb9d0@174.128.233.42:30396?nt=9","enode://0e4aa108144d8678f82306985bf71ab65004bc248f2f239b9fc2085b7daaea8ee317d3178664b04e7104e6a7840a5c61e2f89cf9603ef11e4b5006b5c91dd46d@110.185.107.116:30400?nt=9","enode://644fb6626b80a423371a3b8ac044210c9ce56afc0effc6138bbf85447bc9311f14268974f287dc9a9b575c2b6211f2d8b273ee3580c75b8aceea59c7e4cead7c@23.237.178.90:30396?nt=9"}, "http://service.371738.com/apis/v1"}
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
