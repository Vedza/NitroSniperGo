package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"log"
)

func Ase256(ciphertext []byte, key string, iv string) string {
	block, err := aes.NewCipher([]byte(key[:]))
	if err != nil {
		log.Fatal(err)
	}

	newtext := make([]byte, len(ciphertext))
	dec := cipher.NewCBCDecrypter(block, []byte(iv))
	dec.CryptBlocks(newtext, ciphertext)
	return string(newtext)
}

func MD5(text string) string {
	hash := md5.Sum([]byte(text))
	return string(hash[:])
}

func openSSLKey(password []byte, salt []byte) (string, string) {
	passSalt := string(password) + string(salt)

	result := MD5(passSalt)

	curHash := MD5(passSalt)
	for i := 0; i < 2; i++ {
		cur := MD5(curHash + passSalt)
		curHash = cur
		result += cur
	}
	return result[0 : 4*8], result[4*8 : 4*8+16]
}

func Base64Decode(message []byte) (b []byte, err error) {
	return base64.RawStdEncoding.DecodeString(string(message))
}
