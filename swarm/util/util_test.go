package util

import (
	"os"
	"testing"
)

func TestCipher(t *testing.T) {

	rawData := Defs{ []string{"enode://4e0d8885182bc9a2f23b3525ae0e0daa86068057b4ff7e51dbb4a18696512219c6a62717af7e2d905bd3553662ee1f93caff1942eacc3ebaddfb0e7b5ca475bd@127.0.0.1:30342?nt=9","enode://d3ee0f931d1ea663318516b643ebfafb595b6b56ad6018fab233a33735dce0231d28f434330a25868182a6d1327e59c266c1cbacbefe78fad52afe4473dfbfba@172.16.1.11:30398?nt=9"},"http://172.16.1.10:4000/apis/v2"}

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
