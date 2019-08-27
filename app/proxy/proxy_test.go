package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/extras/jsonrpc"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

var homePageUrls = [110]string{
	"lbry://japanese-sumi-e-ink-painting-scene#42bb7460ba0dc2912e2273f4e828a3852f065428",
	"lbry://scrub-a-dub-dub-pentel-brush-pen-and#e290e2a975e1ddb1b778183f5ea7c91373a1fb48",
	"lbry://rocketbook-an-ocr-update#5a710a74ad9c90eb1161e9351dead2dce0c85cb5",
	"lbry://world-war-1-1914-1918-s-t-51-turn-02-of#021e4fec471661b2f047a0f85ba06732365894a0",
	"lbry://old-school-tactical-a-deadly-race-turn-3#0238d0843c9febe70a1b1eb0e49ea2a4af62e010",
	"lbry://everlast-rocketbook-converting-writing#09a7148cf1ae7c9a900ee4530e8ae24c048b9875",
	"lbry://gear-score-500-heroic-tidal-basin-world#e9d14812ea8a2aeaf77170e24609c38dc9b89e3e",
	"lbry://quick-look-at-d-day-at-iwo-jima#2599d3fd2f7e66d450a13d501a1eb140dc8d7bfb",
	"lbry://bamboo-week-02#28bcc5afefb4b6dafb90d358c6aca7c8eb01d0d4",
	"lbry://quick-look-at-the-kaisers-war-1918-1919#337bfde3eed9a47652edacb1a18d2350dd3020c5",
	"lbry://julian-assange-satoshi-connection#8a2d82af2b0d0fb838ebd8a0d6e1177d33b3638d",
	"lbry://10-best-anime-of-spring-2019-ones-to#a5b70fa9eb07a366c67a1e44c51b40233dc3fe4e",
	"lbry://trump-policies-push-mexicans-to-bitcoin#4d8be9e68a0bb524142a230239ee24dda5344a1a",
	"lbry://cardano-news-ada-crypto-staking-via#29995533d0dac7cd06b1b053c38c22ae9cc26d33",
	"lbry://one#3ae4ed38414e426c29c2bd6aeab7a6ac5da74a98",
	"lbry://two#f5dca9f164da7ac4efd3d6be3f003e1d3ac7ebbe",
	"lbry://japanese-woodland-sumi-e-style-ink#02675c0465ef4ed41024f97942d3477dc1097cdc",
	"lbry://april-2019-q-and-a#b538b7e521452a36bf48332ca64eb1e499a3c15f",
	"lbry://five#7d066008b06b34d203349e385182b6354bee7d5b",
	"lbry://ABoyAndHisDog#278f98e230f0c5965a1682fa007db1e920080c8e",
	"lbry://Crypto101-Oracles#e7231cd1e1cc28361157c5d479d3bd5017f7c851",
	"lbry://Chainlink#922baa51aba0673ba0507be0b46753f276d5b4a2",
	"lbry://JosiahDigibyte#ad2e30d7e35923bbed3b22bf174f141667be7104",
	"lbry://news5#171769ea5f7dc40628bb5b46655d8ce969b81528",
	"lbry://Digibyte#700c40b894735ca841bc4fe2a24f32695251b411",
	"lbry://coinboys#a09858f34acc7e066e862f9410cfa0d63d66d418",
	"lbry://Update-UniversalProtocol#dc9089abcc9bc961da3dec97bd02e0fbb957262f",
	"lbry://update-adbank#5540956afb285ae65ee9f4c3d3ca720d171cacf3",
	"lbry://Enjin#2c8543aad9ffbf09782817b428d485a5a43c5c9a",
	"lbry://cryptoconvos-boxmining#1caca66af70fdd5630c93c26c0faff8a3e4f3e62",
	"lbry://switching-family-members-to-free#e9ee86b7e94b5ab3dccb9325b1c1d95531df4333",
	"lbry://dystopian-hypothetical-free-software#3d8af57b0261b38ce37fcbe7ad9b32db1b671c0a",
	"lbry://what-is-the-worst-linux-distro#d0138f6738010de43269ace609be4326bb9baf93",
	"lbry://on-free-software-and-electric-cars#d74458db710e2d2da2c4af67d14d6744eed8adc3",
	"lbry://rambling-1990s-dial-up-bbs-memories#da1b78b58038fdb0c8288c075d35849592a12dc2",
	"lbry://what-is-the-future-of-various-operating#35bde3877e5765e9f9f74f8f03b50a6abc52f933",
	"lbry://should-we-make-usenet-popular-again#726df122a12216579223d82945091eedd390e164",
	"lbry://is-there-an-all-free-software-or-all#19837914a5e9c4a6922f0b8c6a9078484df0e9cc",
	"lbry://the-eu-says-the-bible-cspan-peter-rabbit#1a7909a8131498d26992ee125e5bfe808b932bee",
	"lbry://does-basic-make-programers-mentally#c7bc3e07227f0aaca0fc97ab975e9b976e91eaf9",
	"lbry://bitcoin-critique-dice-play-the-game#7d9d47a82d3edc03386ebd3acdf9985d25b2e052",
	"lbry://bitcoin-decision-making-how-does-it-work#c1a398995652637d4d85062440323344f4578ab1",
	"lbry://gene-vs-the-automatic-lightswitch#3750894694a62ef6ff8dad4642676a97eb615b5e",
	"lbry://weekly-crypto-recap-kraken-delists-bsv#74fcaec9a5987aa297b4a533e4f926f843fe11a7",
	"lbry://securities-sidetable#e393eaf4b5aff80370202f8cb082120222147231",
	"lbry://bsv-delisted-from-binance-and-shapeshift#0e4287eb95f1dedbfb465f1244cac68b5f9a0356",
	"lbry://crypto-update-will-bitcoin-save-us-2#c2df57e08a81676587e6b405b954d655a5fc40b6",
	"lbry://crypto-update-will-bitcoin-save-us-from#e8f31de0b7d8dc249f6043bbc8b7102e4b7989b0",
	"lbry://weekly-crypto-recap-assange-hodlonaut-vs#3670c96a06e62f3923c773c7611e872a0a251643",
	"lbry://julian-assange-arrested-freedom-of-press#c6d2b56ab792a45e2797284b49fb853000d3ffae",
	"lbry://huge-librem-5-progress-update-with-video#ae94430f85f6577cd2cfe7db4415865843540366",
	"lbry://linus-tech-tips-believes-linux-is-the#bc6f03751cf18542606544c0fd8875405d7e8914",
	"lbry://getting-set-up-with-nextcloud-the-easy#8406189f4ebd6fd5ac83dff69673cc752fc0916c",
	"lbry://here-are-six-reasons-i-love-lutris#90ab36f1f2fb66d37ade7a66dd01d68aa4efedef",
	"lbry://is-valve-s-new-vr-headset-shipping-w#3f2350342dab12da5a82edac0582738d105732fd",
	"lbry://major-proton-steam-play-update-here-s#566d88407c474602bae4665d8676e1f981134f25",
	"lbry://it-s-that-time-of-year-again-the-linux#0db9287ae46419421403696be22d80d537c1b620",
	"lbry://do-you-believe-in-the-work-i-do#92bee7256f9ae900531b10e1f9645d55c83a8f51",
	"lbry://here-s-why-google-stadia-won-t-bring#61e358a416187eddfb7c12054bd79b12a0b65c29",
	"lbry://google-announces-stadia-but-the-question#49dc7594753f7a0fe31ba65fb8dd4eb04112d25f",
	"lbry://single-handed-winner-e-yooso-k700#77385e9cf6aa23519d3c1925faf8789c9adac4e2",
	"lbry://was-i-wrong-about-the-turtle-beach-elite#d8730b0e80cf580f2cae6aaab7a694274347373d",
	"lbry://15-vs-50-microphone-comparison#d10acede3facc29e77bdbc458ca83b94944ed1ba",
	"lbry://oddball-gamdias-e2-mechanical-keyboard#5204c9f700c91f7211697bf17d384cf0a90738d6",
	"lbry://alarm-clock-hd-spy-camera#66749760ba493b480a002e2af37bfdb768ca7e99",
	"lbry://cyberware-3-000-pc-build#f65438934bda03145a369731bbbd9120cee58595",
	"lbry://budget-wireless-microphones-with-one#3da7268ae4941b1bc70611ead22e5ec866e4cb28",
	"lbry://mechanical-keyboard-for-everyone-1st#c4e484009d9f90da09bd3e2a3423ac4433ca678b",
	"lbry://cheap-mouse-with-extra-goodies-gamdias#c27573938bcf399423743dc2e470c8d66d878af9",
	"lbry://keyboard-for-big-boys-drevo-durendal#1f13785a1735565cee9b8427339486933742da63",
	"lbry://monopsony-employers-and-minimum-wages-2#529c5c81688dda6630d680475232d358544b437e",
	"lbry://monopsony-employers-and-minimum-wages#2ec18c02bf339ef08dbe17a8c7b989ca3c132e27",
	"lbry://types-of-price-discrimination#4c40aa8e3c89283666d63e6906fae38aae9ea41e",
	"lbry://rival-and-excludable-goods#f1eeca8dde14f9556707e3181bc36ea69185ebf4",
	"lbry://socially-efficient-and-inefficient#34947603f10f9af68070bf2a93e83a501c1e1d94",
	"lbry://gini-coefficient-and-lorenz-curve#cc9764816f53dfd89d6175c1d75b221447f1b6e8",
	"lbry://shifts-in-the-supply-of-labor#0d5a034b6c63949321a9977e59a788873390f089",
	"lbry://shifts-in-demand-for-labor#633590ddeed94f71d7697744c51273d999ba0146",
	"lbry://cost-minimizing-choice-of-inputs#9c0b16137ed41b82fc4ecd07471a689df8394b44",
	"lbry://hiring-for-monopsony-employer#ca8dd268ebd9068b92599bffa0b0bd6f3e110e9b",
	"lbry://why-do-compressed-air-cans-get-cold#671f4749185e464349713a609be6ec07408b24af",
	"lbry://how-isps-violate-the-laws-of-mathematics#98da3414ade7ed939eb24d943b47a6e18ae5da27",
	"lbry://shells-of-cosmic-time-ft-astrokatie#4a3fa665bfbcb2d922c76f54d149f7c2ba0f8bef",
	"lbry://how-to-make-muons#ce8d0b2c3f32bda425d7309269d5371230d17d02",
	"lbry://hardy-s-paradox-quantum-double-double#e05aedff2bc0e11d8e6644edd4dc377715cac832",
	"lbry://impossible-muons#da41483c75e01a2a47af82430edbcd12e870f2fb",
	"lbry://don-t-forget-to-vote-america#10b5d16ddc7e8e387d8871f0ff2b03ca3e0d6511",
	"lbry://feynman-s-lost-lecture-ft-3blue1brown#720febee2448b5500d3339d0813a6f7b990b7efd",
	"lbry://extraterrestrial-cycloids-why-are-they#08ef9f3fe9a04dc7ad34ab2212acb8a37e5fd865",
	"lbry://how-we-know-black-holes-exist#120e5fadface5f13d8ac8b238ad5b2567555cf92",
	"lbry://massive-raid-delay-no-raid-april-25#d127a7aed25151dff505262c2318d03e680d6058",
	"lbry://why-play-just-buy-a-character-for-100#4c7612049088baea0998d29c07742f1c03003f4b",
	"lbry://insane-floating-mystery-button-hidden#c05df883e1977de0ec75f9bd9e53af04a40bbb5d",
	"lbry://insane-unlimited-loot-glitch-is-back#1489738783564ca4adf30a62f969b1bc30ae9f1d",
	"lbry://unlimited-gear-set-500-weapons-target#5a9767c177fa1a9d0cb94cf672873ee051cd8b85",
	"lbry://insane-instant-1-shot-kill-dark-zone-pvp#5c1d582852b66288ae42d1f79907bdbd72ed4158",
	"lbry://how-have-any-stronghold-invaded-anytime#de90b0068c8f2d7a019b06ce000591172891658e",
	"lbry://how-to-get-multiple-apparel-projects#ca4c21c1a6644f679356f5a0fdf0c35be7bf2d0d",
	"lbry://amazing-dps-exploit-guide-constant-20#dcd105cc5779dbec3a902aa7ec6da1d44beaa65b",
	"lbry://insane-50-cooldown-reduction-on-skills#e6b6fd9d75edd706e19d86fdc34a02cc3568f6ad",
	"lbry://one",
	"lbry://two",
	"lbry://three",
	"lbry://four",
	"lbry://five",
	"lbry://six",
	"lbry://seven",
	"lbry://eight",
	"lbry://nine",
	"lbry://ten",
}

const grumpyServerURL = "http://127.0.0.1:59999"

// A jsonrpc server that responds to every query with an error
func launchGrumpyServer() {
	s := &http.Server{
		Addr: "127.0.0.1:59999",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(jsonrpc.RPCResponse{Error: &jsonrpc.RPCError{Message: "your ways are wrong"}})
			w.Write(response)
		}),
	}
	log.Fatal(s.ListenAndServe())
}

// A shorthand for making a call to proxy function and getting a response
func call(t *testing.T, method string, params ...interface{}) jsonrpc.RPCResponse {
	var (
		response jsonrpc.RPCResponse
		query    *jsonrpc.RPCRequest
	)

	if len(params) > 0 {
		query = jsonrpc.NewRequest(method, params[0])
	} else {
		query = jsonrpc.NewRequest(method)
	}

	rawResponse, err := Proxy(query, "")
	if err != nil {
		t.Fatal(err, rawResponse)
	}

	err = json.Unmarshal(rawResponse, &response)
	if err != nil {
		t.Fatal(err)
	}

	return response
}

// TestForwardCallWithHTTPError tests for HTTP level error connecting to a port that no server is listening on
func TestForwardCall_HTTPError(t *testing.T) {
	config.Override("Lbrynet", "http://127.0.0.1:49999")
	defer config.RestoreOverridden()

	query := jsonrpc.NewRequest(MethodAccountBalance)
	response, err := ForwardCall(*query)
	assert.NotNil(t, err)
	assert.Nil(t, response)
	assert.True(t, strings.HasPrefix(err.Error(), "rpc call account_balance() on http://127.0.0.1:49999"), err.Error())
	assert.True(t, strings.HasSuffix(err.Error(), "connect: connection refused"), err.Error())
}

func TestForwardCall_LbrynetError(t *testing.T) {
	var response jsonrpc.RPCResponse
	query := jsonrpc.NewRequest("crazy_method")
	rawResponse, err := ForwardCall(*query)
	require.Nil(t, err)
	err = json.Unmarshal(rawResponse, &response)
	require.Nil(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Error)
	assert.Equal(t, "Invalid method requested: crazy_method.", response.Error.Message)
}

func TestForwardCall_ClientError(t *testing.T) {
	config.Override("Lbrynet", "http://localhost:59999")
	defer config.RestoreOverridden()

	r := call(t, "anymethod")
	assert.NotNil(t, r.Error)
	assert.Equal(t, "your ways are wrong", r.Error.Message)
}

func TestForwardCall_InvalidResolveParams(t *testing.T) {
	r := call(t, MethodResolve)
	assert.NotNil(t, r.Error)
	assert.Equal(t, "jsonrpc_resolve() missing 1 required positional argument: 'urls'", r.Error.Message)
}

func TestForwardCall_shouldLog(t *testing.T) {
	hook := test.NewLocal(monitor.Logger)

	call(t, MethodResolve, map[string]interface{}{"urls": "what"})
	assert.Equal(t, MethodResolve, hook.LastEntry().Data["method"])
	call(t, MethodAccountBalance)
	assert.Equal(t, MethodResolve, hook.LastEntry().Data["method"])
}

func TestUnmarshalRequest(t *testing.T) {
	_, err := UnmarshalRequest([]byte("yo"))
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "client json parse error: invalid character 'y'"))
}

func TesProxyWithCache(t *testing.T) {
	var (
		err                   error
		query                 *jsonrpc.RPCRequest
		response              jsonrpc.RPCResponse
		rawResponse           []byte
		resolveResponse       *ljsonrpc.ResolveResponse
		cachedResolveResponse *ljsonrpc.ResolveResponse
	)

	resolveArgs := map[string][110]string{paramUrls: homePageUrls}

	query = jsonrpc.NewRequest(MethodResolve, resolveArgs)
	queryBody, _ := json.Marshal(query)
	query, err = UnmarshalRequest(queryBody)
	rawResponse, err = ForwardCall(*query)
	json.Unmarshal(rawResponse, &response)
	if err != nil {
		t.Fatal("failed with an unexpected error", err)
	}
	if response.Error != nil {
		t.Fatal("daemon errored", response.Error.Message)
	}
	response.GetObject(&resolveResponse)
	assert.Equal(t, 110, len(*resolveResponse))

	// Next resolve request should not touch the (grumpy) lbrynet and hit the cache instead
	config.Override("Lbrynet", grumpyServerURL)
	defer config.RestoreOverridden()

	rawResponse, err = ForwardCall(*query)
	json.Unmarshal(rawResponse, &response)
	if err != nil {
		t.Fatal("failed with an unexpected error", err)
	}
	if response.Error != nil {
		t.Fatal("unexpectedly hit the grumpy server")
	}

	response.GetObject(&cachedResolveResponse)
	assert.Equal(t, 110, len(*cachedResolveResponse))
	assert.Equal(t, *resolveResponse, *cachedResolveResponse)
}

func BenchmarkResolve(b *testing.B) {
	query := jsonrpc.NewRequest(MethodResolve, map[string][110]string{paramUrls: homePageUrls})

	wg := sync.WaitGroup{}

	for n := range [100]int{} {
		wg.Add(1)
		go func(n int, wg *sync.WaitGroup) {
			// queryStartTime := time.Now()
			// monitor.Logger.WithFields(log.Fields{"n": n}).Info("sending a request")
			_, err := ForwardCall(*query)
			if err != nil {
				b.Error(err)
				wg.Done()
				return
			}
			// monitor.Logger.WithFields(log.Fields{"n": n}).Info("response fully processed")
			// monitor.LogSuccessfulQuery(fmt.Sprintf("processed a call #%v", n), time.Now().Sub(queryStartTime).Seconds())
			wg.Done()
		}(n, &wg)
	}
	wg.Wait()
}

func BenchmarkDirectResolve(b *testing.B) {
	rpcClient := jsonrpc.NewClient(config.GetLbrynet())
	query := jsonrpc.NewRequest(MethodResolve, map[string][110]string{paramUrls: homePageUrls})

	wg := sync.WaitGroup{}

	for n := range [100]int{} {
		wg.Add(1)
		go func(n int, wg *sync.WaitGroup) {
			queryStartTime := time.Now()
			// monitor.Logger.WithFields(log.Fields{"n": n}).Info("sending a request")
			_, err := rpcClient.CallRaw(query)
			if err != nil {
				b.Error(err)
				wg.Done()
				return
			}
			// monitor.Logger.WithFields(log.Fields{"n": n}).Info("response fully processed")
			monitor.LogSuccessfulQuery(fmt.Sprintf("processed a call #%v", n), time.Now().Sub(queryStartTime).Seconds(), nil)
			wg.Done()
		}(n, &wg)
	}
	wg.Wait()
}
