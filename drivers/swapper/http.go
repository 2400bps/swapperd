package main

import (
	"flag"
	"fmt"
	"log"
	netHttp "net/http"
	"os"
	"os/signal"

	"github.com/btcsuite/btcutil"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/republicprotocol/atom-go/adapters/atoms"
	"github.com/republicprotocol/atom-go/adapters/blockchain/binder"
	btcClient "github.com/republicprotocol/atom-go/adapters/blockchain/clients/btc"
	ethClient "github.com/republicprotocol/atom-go/adapters/blockchain/clients/eth"
	"github.com/republicprotocol/atom-go/adapters/configs/general"
	"github.com/republicprotocol/atom-go/adapters/configs/keystore"
	"github.com/republicprotocol/atom-go/adapters/configs/network"
	"github.com/republicprotocol/atom-go/adapters/http"
	"github.com/republicprotocol/atom-go/adapters/store/leveldb"
	"github.com/republicprotocol/atom-go/services/guardian"
	"github.com/republicprotocol/atom-go/services/store"
	"github.com/republicprotocol/atom-go/services/watch"
)

type watchAdapter struct {
	atoms.AtomBuilder
	binder.Binder
}

func main() {
	port := flag.String("port", "18516", "HTTP Atom port")
	confPath := flag.String("config", os.Getenv("HOME")+"/.swapper/config.json", "Location of the config file")
	keystrPath := flag.String("keystore", os.Getenv("HOME")+"/.swapper/keystore.json", "Location of the keystore file")
	networkPath := flag.String("network", os.Getenv("HOME")+"/.swapper/network.json", "Location of the network file")

	flag.Parse()

	conf, err := config.LoadConfig(*confPath)
	if err != nil {
		panic(err)
	}

	keystr, err := keystore.Load(*keystrPath)
	if err != nil {
		panic(err)
	}

	net, err := network.LoadNetwork(*networkPath)

	db, err := leveldb.NewLDBStore(conf.StoreLocation())
	if err != nil {
		panic(err)
	}
	state := store.NewState(db)

	watcher, err := buildWatcher(net, keystr, state)
	if err != nil {
		panic(err)
	}

	guardian, err := buildGuardian(net, keystr, state)
	if err != nil {
		panic(err)
	}

	errCh1 := watcher.Start()
	watcher.Notify()

	errCh2 := guardian.Start()
	guardian.Notify()

	go func() {
		for err := range errCh1 {
			log.Println("Error :", err)
		}
	}()

	go func() {
		for err := range errCh2 {
			log.Println("Error :", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		_ = <-c
		log.Println("Stopping the swapper service")
		watcher.Stop()
		log.Println("Stopping the guardian service")
		guardian.Stop()
		log.Println("Stopping the atom box safely")
		os.Exit(1)
	}()

	httpAdapter := http.NewBoxHttpAdapter(conf, net, keystr, watcher)
	log.Println(fmt.Sprintf("0.0.0.0:%s", *port))
	log.Fatal(netHttp.ListenAndServe(fmt.Sprintf(":%s", *port), http.NewServer(httpAdapter)))

}

func buildGuardian(net network.Config, kstr keystore.Keystore, state store.State) (guardian.Guardian, error) {
	atomBuilder, err := atoms.NewAtomBuilder(net, kstr)
	if err != nil {
		return nil, err
	}
	return guardian.NewGuardian(atomBuilder, state), nil
}

func buildWatcher(net network.Config, kstr keystore.Keystore, state store.State) (watch.Watch, error) {
	ethConn, err := ethClient.Connect(net)
	if err != nil {
		return nil, err
	}

	btcConn, err := btcClient.Connect(net)
	if err != nil {
		return nil, err
	}

	ethKey, err := kstr.GetKey(1, 0)
	if err != nil {
		return nil, err
	}

	btcKey, err := kstr.GetKey(0, 0)
	if err != nil {
		return nil, err
	}

	_WIF := btcKey.GetKeyString()
	if err != nil {
		return nil, err
	}

	WIF, err := btcutil.DecodeWIF(_WIF)
	if err != nil {
		return nil, err
	}

	err = btcConn.Client.ImportPrivKey(WIF)
	if err != nil {
		return nil, err
	}

	privKey, err := ethKey.GetKey()
	if err != nil {
		return nil, err
	}
	owner := bind.NewKeyedTransactor(privKey)
	owner.GasLimit = 3000000

	ethBinder, err := binder.NewBinder(privKey, ethConn)

	atomBuilder, err := atoms.NewAtomBuilder(net, kstr)
	wAdapter := watchAdapter{
		atomBuilder,
		ethBinder,
	}

	watcher := watch.NewWatch(&wAdapter, state)
	return watcher, nil
}