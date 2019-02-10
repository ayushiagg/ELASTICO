package main

import (
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"reflect"
	"math/big"

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
	var v *big.Int
	v.SetUint64(8)

	var prime1, _ = new(big.Int).SetString("256", 10)
	// Generate random numbers in range [0..prime1]
	// Ignore error values
	// Don't use this code to generate secret keys that protect important stuff!
	x, _ := rand.Int(rand.Reader, prime1)
	y, _ := rand.Int(rand.Reader, prime1)
	fmt.Printf("x: %v\n", x)
	fmt.Printf("y: %v\n", y)

}
