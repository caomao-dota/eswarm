package util

import (
	"os"
	"testing"
)

func TestCipher(t *testing.T) {

	rawData := Defs{ []string{"enode://e7f6cffab1841d4fdc118f1e21947b6d71cb29f663340fb817a266c3d42a2af9a0d350b247c619e64c528b97932b3f14e7f60643d4db232491c72a22a9d9b04a@110.185.107.116:30400?nt=9"},"http://124.156.115.14:4000/apis/v2"}

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
