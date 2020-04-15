package publish

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// CreatePublishRequest creates and returns a HTTP request providing data for the publishing endpoint.
func CreatePublishRequest(t *testing.T, data []byte) *http.Request {
	readSeeker := bytes.NewReader(data)
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	fileBody, err := writer.CreateFormFile(fileFieldName, "lbry_auto_test_file")
	require.NoError(t, err)
	_, err = io.Copy(fileBody, readSeeker)
	require.NoError(t, err)

	jsonPayload, err := writer.CreateFormField(jsonRPCFieldName)
	require.NoError(t, err)
	jsonPayload.Write([]byte(expectedStreamCreateRequest))

	writer.Close()

	req, err := http.NewRequest("POST", "/api/v1/proxy", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

var expectedStreamCreateRequest = `
{
    "id": 1567580184168,
    "jsonrpc": "2.0",
    "method": "stream_create",
    "params": {
        "name": "test",
        "title": "test",
        "description": "test description",
        "bid": "0.10000000",
        "languages": [
            "en"
        ],
        "tags": [],
        "thumbnail_url": "http://smallmedia.com/thumbnail.jpg",
        "license": "None",
        "release_time": 1567580184,
		"wallet_id": "%s",
        "file_path": "%s"
    }
}`

var expectedStreamCreateResponse = `
{
	"id": 0,
	"jsonrpc": "2.0",
	"result": {
	  "height": -2,
	  "hex": "0100000001b25ac56e2fda6353b732863e338e205a19d1d2f4e38145048ee501e373fd8585010000006a4730440220205c1cea74188145c8d3200ef2914b5852c8a3b151876c9d9431e9b52e82b3e0022061169e87088e2fd0759d457d0a444a9445d404b64358d5cbac08c5ab950dca6c012103ebc2c0ec16d9e24b5ebcb4bf957ddc9fd7a80376d1cff0d79f5d65e381d7fe42ffffffff0200e1f50500000000fddc01b50b626c616e6b2d696d6167654db1010127876157202060e91daaf771f57c2b78c254f9cb24eda15eb1995dfe4ea874fa93396c62e1fe82612e6b9b786ea0c55166e98e7880da5e3b48ef29ab4d1a9c83f71482c22a4acad548c27a5f5643550d0434f3b00ae6010a82010a306c7df435d412c603390f593ef658c199817c7830ba3f16b7eadd8f99fa50e85dbd0d2b3dc61eadc33fe096e3872d1545120f746d706e6b745f343962712e706e6718632209696d6167652f706e673230eda7090b2d59beb0d77de489961cb73bbc73bbbb80d2c3c0e5f547b8c07dc0eded9627ce12872ca86a20a51d54ae3c4b120650696361736f1a0d5075626c696320446f6d61696e2218687474703a2f2f7075626c69632d646f6d61696e2e6f72672880f1c3ea053222080112196f147b27d1c70b5fb7ff1560d32bfda68507a89a0f214e74e0188087a70e520408051007420b426c616e6b20496d6167654a184120626c616e6b20504e472074686174206973203578372e52252a23687474703a2f2f736d616c6c6d656469612e636f6d2f7468756d626e61696c2e6a70675a05626c616e6b5a03617274620208016a1308ec0112024e481a0a4d616e636865737465726d7576a914147b27d1c70b5fb7ff1560d32bfda68507a89a0f88acac5e7d1d000000001976a914d7d23f1f17bdd156052ea8c496a95070157fb6ab88ac00000000",
	  "inputs": [
		{
		  "address": "n4SAW6U5NeYRqQTdos4cLMgtbWRBFW8X16",
		  "amount": "5.969662",
		  "confirmations": 2,
		  "height": 213,
		  "is_change": true,
		  "is_mine": true,
		  "nout": 1,
		  "timestamp": 1565587608,
		  "txid": "8585fd73e301e58e044581e3f4d2d1195a208e333e8632b75363da2f6ec55ab2",
		  "type": "payment"
		}
	  ],
	  "outputs": [
		{
		  "address": "mhPFLtT7YzmNfMuQYr4PQXAJdtaTKWRLFy",
		  "amount": "1.0",
		  "claim_id": "5cfb92c3e6a80aedee5282c3f64b565bc6965562",
		  "claim_op": "create",
		  "confirmations": -2,
		  "height": -2,
		  "is_channel_signature_valid": true,
		  "meta": {},
		  "name": "blank-image",
		  "normalized_name": "blank-image",
		  "nout": 0,
		  "permanent_url": "lbry://blank-image#5cfb92c3e6a80aedee5282c3f64b565bc6965562",
		  "signing_channel": {
			"address": "mvE3pR2rH5mP1Hx8UEipnPt3Atp89tXqVw",
			"amount": "1.0",
			"claim_id": "cbf954c2782b7cf571f7aa1de960202057618727",
			"claim_op": "update",
			"confirmations": 5,
			"height": 210,
			"is_change": false,
			"is_mine": true,
			"meta": {},
			"name": "@channel",
			"normalized_name": "@channel",
			"nout": 0,
			"permanent_url": "lbry://@channel#cbf954c2782b7cf571f7aa1de960202057618727",
			"timestamp": 1565587607,
			"txid": "794fc94e7ac645d5fc06c14e5ac9be9d9afa53cd540a349ee276662b23e21396",
			"type": "claim",
			"value": {
			  "public_key": "3056301006072a8648ce3d020106052b8104000a0342000404b644588c6a32f425fa8c2c3b0404898c79d405d1e90783adcf9a2bdbad505012f1e6be38f7837b69d5f2a1a1959135701780f01fc91c396158c4b1b9b1e304",
			  "public_key_id": "mrPWGtFam2wwv7D1QRgXXrXePLqUGdKaCb",
			  "title": "New Channel"
			},
			"value_type": "channel"
		  },
		  "timestamp": null,
		  "txid": "474e26f1aceebbdbbbad02afd37dd39aa3eb221098fa8a4073b1117264422e98",
		  "type": "claim",
		  "value": {
			"author": "Picaso",
			"description": "A blank PNG that is 5x7.",
			"fee": {
			  "address": "mhPFLtT7YzmNfMuQYr4PQXAJdtaTKWRLFy",
			  "amount": "0.3",
			  "currency": "LBC"
			},
			"image": {
			  "height": 7,
			  "width": 5
			},
			"languages": [
			  "en"
			],
			"license": "Public Domain",
			"license_url": "http://public-domain.org",
			"locations": [
			  {
				"city": "Manchester",
				"country": "US",
				"state": "NH"
			  }
			],
			"release_time": "1565587584",
			"source": {
			  "hash": "6c7df435d412c603390f593ef658c199817c7830ba3f16b7eadd8f99fa50e85dbd0d2b3dc61eadc33fe096e3872d1545",
			  "media_type": "image/png",
			  "name": "tmpnkt_49bq.png",
			  "sd_hash": "eda7090b2d59beb0d77de489961cb73bbc73bbbb80d2c3c0e5f547b8c07dc0eded9627ce12872ca86a20a51d54ae3c4b",
			  "size": "99"
			},
			"stream_type": "image",
			"tags": [
			  "blank",
			  "art"
			],
			"thumbnail": {
			  "url": "http://smallmedia.com/thumbnail.jpg"
			},
			"title": "Blank Image"
		  },
		  "value_type": "stream"
		},
		{
		  "address": "n1C7SV6XSvTgHK84pMQ23KZLszCsm53T3Q",
		  "amount": "4.947555",
		  "confirmations": -2,
		  "height": -2,
		  "nout": 1,
		  "timestamp": null,
		  "txid": "474e26f1aceebbdbbbad02afd37dd39aa3eb221098fa8a4073b1117264422e98",
		  "type": "payment"
		}
	  ],
	  "total_fee": "0.022107",
	  "total_input": "5.969662",
	  "total_output": "5.947555",
	  "txid": "474e26f1aceebbdbbbad02afd37dd39aa3eb221098fa8a4073b1117264422e98"
	}
  }
`
