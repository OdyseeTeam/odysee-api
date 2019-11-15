package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/models"
	"github.com/lbryio/lbrytv/util/wallet"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/spf13/cobra"
	"github.com/volatiletech/sqlboiler/boil"
)

func init() {
	rootCmd.AddCommand(migrateToWallets)
}

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func ptrToBool(b bool) *bool { return &b }

var migrateToWallets = &cobra.Command{
	Use:    "migrate_to_wallets",
	Short:  "Migrate existing accounts to wallets",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		realRun := false
		if len(args) > 0 && args[0] == "doit" {
			realRun = true
		}
		lbrynetRouter := router.New(config.GetLbrynetServers())
		c := ljsonrpc.NewClient(lbrynetRouter.GetBalancedSDKAddress())

		users, err := models.Users(models.UserWhere.WalletID.EQ("")).AllG()
		if err != nil {
			panic(err)
		}
		fmt.Printf("%v users without wallets\n", len(users))

		for _, u := range users {
			wid := wallet.MakeID(u.ID)

			if realRun {
				_, err = c.WalletCreate(wid, &ljsonrpc.WalletCreateOpts{CreateAccount: false})
				if err != nil {
					panic(err)
				}
			}

			u.WalletID = wid

			if realRun {
				_, err = u.UpdateG(boil.Infer())
				if err != nil {
					panic(err)
				}
			}

			fmt.Printf("initialized wallet for user %v (wid=%v)\n", u.ID, wid)

			accID := u.SDKAccountID.String

			channels, err := c.ChannelList(&accID, 1, 999, nil)
			if channels.TotalPages > 1 {
				panic(fmt.Sprintf("expected one page, found %v for account %v", channels.TotalPages, accID))
			}

			fmt.Printf("got %v channels for acc id=%v, uid=%v\n", len(channels.Items), accID, u.ID)

			for _, channel := range channels.Items {
				key, err := c.ChannelExport(channel.ClaimID, nil, nil)
				if err != nil {
					panic(err)
				}
				keyS := string(*key)

				if realRun {
					_, err = c.ChannelImport(keyS, &wid)
					if err != nil {
						panic(err)
					}
				}
				fmt.Printf("exported channel %v to wallet %v\n", keyS, wid)
			}

			var acc *ljsonrpc.Account
			if realRun {
				acc, err = c.AccountRemove(accID)
				if err != nil {
					panic(err)
				}
				fmt.Printf("removed account %v for user %v (seed=%v)\n", accID, u.ID, acc.Seed)
			}

			newAccName := lbrynet.MakeAccountName(u.ID)

			var newAcc *ljsonrpc.Account
			if realRun {
				newAcc, err = c.AccountAdd(newAccName, acc.Seed, nil, nil, ptrToBool(true), &wid)
				if err != nil {
					prettyPrint(acc)
					panic(err)
				}
				fmt.Printf("DONE migrating account %v to wallet %v (acc id=%v)\n", accID, wid, newAcc.ID)
			} else {
				fmt.Printf("TESTED migrating account %v to wallet %v\n", accID, wid)
			}

		}
	},
}
