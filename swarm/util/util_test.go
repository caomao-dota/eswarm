package util

import (
	"os"
	"testing"
)

func TestCipher(t *testing.T) {

	rawData := Defs{ []string{"enode://db0aa64d8bac5d95a112f344a9092d60e6fb8a6e5165af1a0752db27ed5465d3b459896ac92bc15ab981fc96b19f6103f906199858990afa4ced7ab9dc7829fe@127.0.0.1:30342?nt=5"},"http://172.16.1.10:4000/apis/v2"}

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

	def,err := DecipherData(result)
	if err == nil {
		t.Log("result:",def)
	}else{
		t.Error("error in cipher/decipher",err)
	}
}
