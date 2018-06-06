package regtest

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/republicprotocol/atom-go/adapters/btc"
)

// Start a local Ganache instance.
func Start() *exec.Cmd {
	cmd := exec.Command("bitcoind", "--regtest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	return cmd
}

func Stop(cmd *exec.Cmd) {
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

func Mine(connection btc.Connection) error {

	fmt.Println("initial 100")
	_, err := connection.Client.Generate(100)
	if err != nil {
		return err
	}
	fmt.Println("100 done")

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			_, err := connection.Client.Generate(1)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func NewAccount(connection btc.Connection, name string, value btcutil.Amount) (btcutil.Address, error) {
	addr, err := connection.Client.GetAccountAddress(name)
	if err != nil {
		return nil, err
	}

	if value > 0 {
		_, err = connection.Client.SendToAddress(addr, value)
		if err != nil {
			return nil, err
		}

		_, err = connection.Client.Generate(1)
		if err != nil {
			return nil, err
		}
	}

	return addr, nil
}

// func NewAccount(conn client.btc.Connection, eth *big.Int) (*bind.TransactOpts, common.Address, error) {
// 	ethereumPair, err := crypto.GenerateKey()
// 	if err != nil {
// 		return nil, common.Address{}, err
// 	}
// 	addr := crypto.PubkeyToAddress(ethereumPair.PublicKey)
// 	account := bind.NewKeyedTransactor(ethereumPair)
// 	if eth.Cmp(big.NewInt(0)) > 0 {
// 		if err := DistributeEth(conn, addr); err != nil {
// 			return nil, common.Address{}, err
// 		}
// 	}

// 	return account, addr, nil
// }
