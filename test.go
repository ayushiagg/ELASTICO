package main

import (
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"reflect"
	"math/big"
	"strconv"

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
	n:= 2345
	str:= strconv.Itoa(n)
	var prime1, _ = new(big.Int).SetString(str, 10)
	// Generate random numbers in range [0..prime1]
	// Ignore error values
	x, _ := rand.Int(rand.Reader, prime1)
	
	fmt.Printf("x: %v\n", x)
	

}
