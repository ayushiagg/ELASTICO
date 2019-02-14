package main

import (
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"reflect"
	"math/big"
	"strconv"

)

type Identity struct{
	IP string
	PK string
	committee_id int
	PoW map[string]interface{}
	epoch_randomness string
	port int
}

type Elastico struct{
	key *rsa.PrivateKey
	IP string
	cur []Identity
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

	mape := make(map[string]interface{})
	mape["hash"] = ""
	mape["set_of_Rs"] = ""
	mape["nonce"] = 0
	fmt.Printf("x: %v\n", x)
	
	var cure []Identity

	a:= make(map[int]map[string]string)
	a[0] = make(map[string]string)
	a[0]["d"] = "dfsgf"
	a[0]["de"] = "dfsgf"
	fmt.Println(mape, cure, a)

}
