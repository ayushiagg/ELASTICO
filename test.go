package main

import (
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"reflect"

)
type Elastico struct{
	key *rsa.PrivateKey
	IP string
}
func main() {
	e:= Elastico{}
	var err error 
	e.key,err = rsa.GenerateKey(rand.Reader, 2048)
	if err!= nil{
		fmt.Println(e.key)
	}
	fmt.Println(reflect.TypeOf(e.key))

	c := 4
	i := make([]byte, c)
	_, err = rand.Read(i)
	if err != nil {
		fmt.Println("error:", err.Error)
	}
	e.IP= fmt.Sprintf("%v.%v.%v.%v" , i[0] , i[1], i[2], i[3])
	fmt.Println(e.IP)
}
