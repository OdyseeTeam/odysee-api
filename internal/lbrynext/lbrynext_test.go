package lbrynext

import (
	"strings"
	"testing"

	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/test"

	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func Test_compareResponses(t *testing.T) {
	r := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}
	xr := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "not ok"}}
	_, _, diff := compareResponses(r, xr)
	assert.Contains(t, diffPlainText(diff), `"ok": "+>>not <<+ok"`)
}

func Test_compareResponses_Match(t *testing.T) {
	r := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}
	xr := &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}
	_, _, diff := compareResponses(r, xr)
	assert.Nil(t, diff)
}

func TestInstallHooks_ResponseMatch(t *testing.T) {
	hook := logrusTest.NewLocal(logger.Entry.Logger)

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	reqChanX := test.ReqChan()
	srvX := test.MockHTTPServer(reqChanX)
	defer srvX.Close()

	srv.NextResponse <- resolveResponse
	srvX.NextResponse <- resolveResponse

	c := query.NewCaller(srv.URL, 0)
	config.Override("ExperimentalLbrynetServer", srvX.URL)
	defer config.RestoreOverridden()
	propagationPct = 100
	defer func() { propagationPct = 10 }()

	InstallHooks(c)
	request := jsonrpc.NewRequest(query.MethodResolve, map[string]interface{}{"urls": "what"})
	c.Call(request)

	receivedRequest := <-reqChan
	expectedRequest := test.ReqToStr(t, request)
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)

	receivedRequestX := <-reqChanX
	expectedRequestX := test.ReqToStr(t, request)
	assert.EqualValues(t, expectedRequestX, receivedRequestX.Body)

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Contains(t, entry.Message, "experimental call succeeded")
	assert.Equal(t, query.MethodResolve, entry.Data["method"])
}

func TestInstallHooks_DifferentResponse(t *testing.T) {
	hook := logrusTest.NewLocal(logger.Entry.Logger)

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	reqChanX := test.ReqChan()
	srvX := test.MockHTTPServer(reqChanX)
	defer srvX.Close()

	srv.NextResponse <- resolveResponse
	// Tweak resolve response
	srvX.NextResponse <- strings.Replace(resolveResponse, "d66f8ba85c85ca48daba9183bd349307fe30cb43", "abcdef", 1)

	c := query.NewCaller(srv.URL, 0)
	config.Override("ExperimentalLbrynetServer", srvX.URL)
	defer config.RestoreOverridden()
	propagationPct = 100
	defer func() { propagationPct = 10 }()

	InstallHooks(c)
	request := jsonrpc.NewRequest(query.MethodResolve, map[string]interface{}{"urls": "what"})
	_, err := c.Call(request)
	require.NoError(t, err)

	receivedRequest := <-reqChan
	expectedRequest := test.ReqToStr(t, request)
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)

	receivedRequestX := <-reqChanX
	expectedRequestX := test.ReqToStr(t, request)
	assert.EqualValues(t, expectedRequestX, receivedRequestX.Body)

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Contains(t, entry.Message, "experimental call result differs")
	assert.Equal(t, query.MethodResolve, entry.Data["method"])
}

var resolveResponse = `
{
  "jsonrpc": "2.0",
  "result": {
    "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43": {
      "address": "bWczbT1P6JQQ63PiDvFiYbkRYpQs6h6oap",
      "amount": "0.1",
      "canonical_url": "lbry://@Bombards_Body_Language#f/Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d",
      "claim_id": "d66f8ba85c85ca48daba9183bd349307fe30cb43",
      "claim_op": "update",
      "confirmations": 14930,
      "height": 752080,
      "is_channel_signature_valid": true,
      "meta": {
        "activation_height": 752069,
        "creation_height": 752069,
        "creation_timestamp": 1587493237,
        "effective_amount": "0.1",
        "expiration_height": 2854469,
        "is_controlling": true,
        "reposted": 4,
        "support_amount": "0.0",
        "take_over_height": 752069,
        "trending_global": 0.0,
        "trending_group": 0,
        "trending_local": 0.0,
        "trending_mixed": 0.0
      },
      "name": "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis",
      "normalized_name": "body-language---robert-f.-kennedy-assassination---hypnosis",
      "nout": 0,
	  "permanent_url": "lbry://Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43",
	  "protobuf": "0109675c0ab3bb225f9b56e94df27cc3e073d899f3e1cc696925f6f820375292404447f6c8b61214df0444994f0458042ea95a37b531ebcd6b3dd6092914c78270a197b07909382031efcf7d1c32c7d8c27ac526740af4010ab5010a30fae1e6db07c03a857f526ae9956d80be64dd95b85eeb79560d5f0fb8aea6e70531f089587f946f8916f42052abdb4fb2123e426f6479204c616e6775616765202d20526f6265727420462e204b656e6e65647920417373617373696e6174696f6e2026204879706e6f7369732e6d703418ed9c9e97022209766964656f2f6d7034323051ee258ebbe33c15d37a28e90b1ba1e9ddfddd277bede52bd59431ce1b6ed6475f6c2c7299210a98eb3b746cbffa1f941a044e6f6e6528caa1fdf40532230801121955c4425439537bf7f8c0c1dca66490826e90dfffdeaa6b54891880f4f6905d5a0908800f10b80818e00b423a426f6479204c616e6775616765202d20526f6265727420462e204b656e6e65647920417373617373696e6174696f6e2026204879706e6f7369734ace0254686973206973206f6e65206f66206d7920706572736f6e616c206661766f75726974657321200a0a546f2068656c7020737570706f72742074686973206368616e6e656c20616e6420746f206c6561726e206d6f72652061626f757420626f6479206c616e67756167652c20596f752063616e207669736974206d79207765627369746520776865726520796f752063616e2076696577206578636c757369766520636f6e74656e742c2061732077656c6c2061732061207475746f7269616c207365726965732074686174206578706c61696e73206d79206d6574686f647320696e206d6f72652064657461696c2e0a0a68747470733a2f2f626f6d6261726473626f64796c616e67756167652e636f6d2f0a0a4e6f74653a20416c6c20636f6d6d656e747320696e206d7920766964656f7320617265207374726963746c79206d79206f70696e696f6e2e52312a2f68747470733a2f2f737065652e63682f302f4556544d59534566304f4c75766a6b4d475272464875626c2e6a7065675a0d617373617373696e6174696f6e5a0d626f6479206c616e67756167655a09656475636174696f6e5a086879706e6f7369735a076b656e6e65647962020801",
      "purchase_receipt": null,
      "short_url": "lbry://Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d",
      "signing_channel": {
        "address": "bJ5oueNUmpPpHkK3dEBtmdqy1dGyTmJgiq",
        "amount": "800.0",
        "canonical_url": "lbry://@Bombards_Body_Language#f",
        "claim_id": "f399d873e0c37cf24de9569b5f22bbb30a5c6709",
        "claim_op": "update",
        "confirmations": 19240,
        "has_signing_key": false,
        "height": 747770,
        "meta": {
          "activation_height": 687996,
          "claims_in_channel": 253,
          "creation_height": 687996,
          "creation_timestamp": 1577197630,
          "effective_amount": "2969.71",
          "expiration_height": 2790396,
          "is_controlling": true,
          "reposted": 0,
          "support_amount": "2169.71",
          "take_over_height": 687996,
          "trending_global": 0.0,
          "trending_group": 0,
          "trending_local": 0.0,
          "trending_mixed": -20.426517486572266
        },
        "name": "@Bombards_Body_Language",
        "normalized_name": "@bombards_body_language",
        "nout": 0,
        "permanent_url": "lbry://@Bombards_Body_Language#f399d873e0c37cf24de9569b5f22bbb30a5c6709",
        "short_url": "lbry://@Bombards_Body_Language#f",
        "timestamp": 1586802450,
        "txid": "36d7a1495102ff3b91fe26f255b9403b9e25fe16c869af71adc941ad39167b77",
        "type": "claim",
        "value": {
          "cover": {
            "url": "https://spee.ch/1/dcc5f235-a895-4c8b-9e61-2177449b96c4.jpg"
          },
          "description": "This is a channel dedicated to helping people see the corruption and deception of public figures using body language analysis.\nTo help support this channel and to learn more about body language, You can visit my [website](https://bombardsbodylanguage.com/) where you can view exclusive content, as well as a tutorial series that explains my methods in more detail.\n\n",
          "public_key": "3056301006072a8648ce3d020106052b8104000a034200041633f79926012767fe36a84c11dd7d66050c796bfdb26dc66599e5612b9bbce819e46df10a54ad67bdce1ae42455d5e60995eccbc7a013e72913553140187e30",
          "public_key_id": "baKc1SpWE3XqH4auz2C9a7eUhQ1G2XE76R",
          "tags": [
            "body language",
            "bombards",
            "education",
            "ghost",
            "news",
            "politics"
          ],
          "thumbnail": {
            "url": "https://spee.ch/6/c33bdd7f-3f0d-4f93-a275-5e9ad238f673.jpeg"
          },
          "title": "Bombards Body Language",
          "website_url": "https://bombardsbodylanguage.com/"
        },
        "value_type": "channel"
      },
      "timestamp": 1587495005,
      "txid": "a6005c8b55122eb1663041362546928e5961a037882fa04d52e70c190324ee64",
      "type": "claim",
      "value": {
        "description": "This is one of my personal favourites! \n\nTo help support this channel and to learn more about body language, You can visit my website where you can view exclusive content, as well as a tutorial series that explains my methods in more detail.\n\nhttps://bombardsbodylanguage.com/\n\nNote: All comments in my videos are strictly my opinion.",
        "fee": {
          "address": "bWczbT1P6JQQ63PiDvFiYbkRYpQs6h6oap",
          "amount": "250",
          "currency": "LBC"
        },
        "languages": [
          "en"
        ],
        "license": "None",
        "release_time": "1587499210",
        "source": {
          "hash": "fae1e6db07c03a857f526ae9956d80be64dd95b85eeb79560d5f0fb8aea6e70531f089587f946f8916f42052abdb4fb2",
          "media_type": "video/mp4",
          "name": "Body Language - Robert F. Kennedy Assassination \u0026 Hypnosis.mp4",
          "sd_hash": "51ee258ebbe33c15d37a28e90b1ba1e9ddfddd277bede52bd59431ce1b6ed6475f6c2c7299210a98eb3b746cbffa1f94",
          "size": "585600621"
        },
        "stream_type": "video",
        "tags": [
          "assassination",
          "body language",
          "education",
          "hypnosis",
          "kennedy"
        ],
        "thumbnail": {
          "url": "https://spee.ch/0/EVTMYSEf0OLuvjkMGRrFHubl.jpeg"
        },
        "title": "Body Language - Robert F. Kennedy Assassination \u0026 Hypnosis",
        "video": {
          "duration": 1504,
          "height": 1080,
          "width": 1920
        }
      },
      "value_type": "stream"
    }
  },
  "id": 0
}
`
