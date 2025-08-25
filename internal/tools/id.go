package tools

import (
	"bytes"
	"crypto/rand"
)

const (
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	idLength = 32
)

func GenerateRandomID() (string, error) {
	// 创建指定长度的字节切片
	p := make([]byte, idLength)
	if _, err := rand.Read(p); err != nil {
		return "", err
	}

	// 将每个字节转为字母表中的字符
	for i, b := range p {
		p[i] = alphabet[b%byte(len(alphabet))]
	}
	p[0] = bytes.ToUpper([]byte{p[0]})[0]
	return string(p), nil
}
