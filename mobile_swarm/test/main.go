package main

import (
	"fmt"
	"github.com/plotozhu/MDCMainnet/mobile_swarm"
	"time"
)

func main() {

	path := "/Users/sean/Documents/swarm"
	//bootnode := "enode://156de950834f68902c2029a7efbf3f653e3ac924da0b4848819f61203dcd5aa774838e6d66df8da6108c1a1462cfdaaf0645ab9849c29ffd5641739684551457@172.16.1.11:30398"
	//bootnode := "enode://4b0581175812b8c15aa43f148f61af9c8a0d903cc5a26249e7400cda9ac6f0db35b4dbdf2423702fd693a0c6b75c05209709273379363d6fe742b70ec58bfd97@124.156.115.14:30399"
	//bootnode := "enode://e7f6cffab1841d4fdc118f1e21947b6d71cb29f663340fb817a266c3d42a2af9a0d350b247c619e64c528b97932b3f14e7f60643d4db232491c72a22a9d9b04a@110.185.107.116:30400"
	//err := eswarm.Clean(path, 1)
	//if err != nil { return }

	expireTime, err := csdc.Activate(path, "5ca31612427e9a564c2089c2", "123", "http://124.156.115.14:4000/apis/v1/activate", false, "123", 1)
	if err != nil {
		fmt.Println(expireTime, err)
		//return
	}
	fmt.Println(expireTime)

	_, err = csdc.Start(path, "123", "https://raw.githubusercontent.com/wiki/CSDCFund/csdc/nodeslist.txt", "")
	if err != nil {
		fmt.Println(err)
		return
	}

	time.Sleep(3600 * time.Second)

}
