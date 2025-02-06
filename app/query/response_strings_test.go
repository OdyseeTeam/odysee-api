package query

var resolveResponseWithPurchase = `
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
      "purchase_receipt": {
        "address": "bWczbT1P6JQQ63PiDvFiYbkRYpQs6h6oap",
        "amount": "250.0",
        "claim_id": "d66f8ba85c85ca48daba9183bd349307fe30cb43",
        "confirmations": 11393,
        "height": 755617,
        "nout": 0,
        "timestamp": 1588063350,
        "txid": "ff990688df370072f408e2db9d217d2cf331d92ac594d5e6e8391143e9d38160",
        "type": "purchase"
      },
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

var resolveResponseWithoutPurchase = `
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

var purchaseCreateResponse = `
{
  "jsonrpc": "2.0",
  "result": {
	"height": -2,
	"hex": "010000000103b0195dae4375272e8cb1a2d3d10495561b60ca973623d996c2c71b2a2fb62d020000006b483045022100f031ff1ea54da7172fc9bcca7279fd2c1c5ade9681838fce3892f89a3072386802203526c6664b181d4f3fe416b4cb2e5bcc9fe24380bb3c60fa43143c51daab09f2012103d5fd7f40a66a941ff50108f7d1d75089cbf8fe6ca2217f293eaf399b51b9375cffffffff030065cd1d000000001976a914043dbb33a8f732d375bda614f12f67b6f2e1467788ac0000000000000000196a17500a14de5b952438d94089d17140fd0779031421fc98ea3cf3ef08000000001976a9140ed27f29d571303273bfec053ab6e34ea696cc2488ac00000000",
	"inputs": [
	  {
		"address": "bE5eKV8DvMDiEt6dRcgtYHfxajDS5xsgsv",
		"amount": "6.499572",
		"confirmations": 319,
		"height": 767156,
		"nout": 2,
		"timestamp": 1589930795,
		"txid": "2db62f2a1bc7c296d9233697ca601b569504d1d3a2b18c2e277543ae5d19b003",
		"type": "payment"
	  }
	],
	"outputs": [
	  {
		"address": "bD7hMAHrxvFYiXeDAu3HNoCVLLRiKKzAES",
		"amount": "5.0",
		"confirmations": -2,
		"height": -2,
		"nout": 0,
		"timestamp": null,
		"txid": "dd22526f95914245ecd39a243e1602d2a2117d8a3725be7adea1ad98ba143bf5",
		"type": "payment"
	  },
	  {
		"address": null,
		"amount": "0.0",
		"confirmations": -2,
		"height": -2,
		"nout": 1,
		"timestamp": null,
		"txid": "dd22526f95914245ecd39a243e1602d2a2117d8a3725be7adea1ad98ba143bf5",
		"type": "data"
	  },
	  {
		"address": "bE5eKV8DvMDiEt6dRcgtYHfxajDS5xsgsv",
		"amount": "1.499431",
		"confirmations": -2,
		"height": -2,
		"nout": 2,
		"timestamp": null,
		"txid": "dd22526f95914245ecd39a243e1602d2a2117d8a3725be7adea1ad98ba143bf5",
		"type": "payment"
	  }
	],
	"total_fee": "0.000141",
	"total_input": "6.499572",
	"total_output": "6.499431",
	"txid": "dd22526f95914245ecd39a243e1602d2a2117d8a3725be7adea1ad98ba143bf5"
  },
  "id": 0
}
`

var purchaseCreateExistingResponse = `
{
  "jsonrpc": "2.0",
  "error": {
	"code": -32500,
	"data": {
	  "args": [],
	  "command": "purchase_create",
	  "kwargs": {
		"allow_duplicate_purchase": false,
		"blocking": false,
		"claim_id": "f2b88f9ed44bf722175a64def450652391b814e8",
		"funding_account_ids": [],
		"override_max_key_fee": false,
		"preview": false,
		"wallet_id": "lbrytv-id.1763208.wallet"
	  },
	  "name": "Exception",
	  "traceback": [
		"Traceback (most recent call last):",
		"  File \"lbry/extras/daemon/daemon.py\", line 698, in _process_rpc_call",
		"  File \"lbry/extras/daemon/daemon.py\", line 2239, in jsonrpc_purchase_create",
		"Exception: You already have a purchase for claim_id 'f2b88f9ed44bf722175a64def450652391b814e8'. Use --allow-duplicate-purchase flag to override.",
		""
	  ]
	},
	"message": "You already have a purchase for claim_id 'f2b88f9ed44bf722175a64def450652391b814e8'. Use --allow-duplicate-purchase flag to override."
  },
  "id": 0
}
`

var purchaseCreateFreeResponse = `
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32500,
    "message": "Claim 'None' does not have a purchase price.",
    "data": {
      "args": [],
      "command": "purchase_create",
      "kwargs": {
        "url": "one",
        "wallet_id": "lbrytv-id.779768.wallet"
      },
      "name": "Exception",
      "traceback": [
        "Traceback (most recent call last):",
        "  File \"lbry/extras/daemon/daemon.py\", line 698, in _process_rpc_call",
        "  File \"lbry/extras/daemon/daemon.py\", line 2244, in jsonrpc_purchase_create",
        "Exception: Claim 'None' does not have a purchase price.",
        ""
      ]
    }
  },
  "id": 0
}
`

var resolveResponseFree = `
{
	"jsonrpc": "2.0",
	"result": {
	  "what": {
		"address": "bMAhbSTxw9of4zPngbhednea6ajKxd3GnS",
		"amount": "0.001",
		"canonical_url": "lbry://what#1",
		"claim_id": "19b9c243bea0c45175e6a6027911abbad53e983e",
		"claim_op": "create",
		"confirmations": 380546,
		"height": 387055,
		"meta": {
		  "activation_height": 390588,
		  "creation_height": 387055,
		  "creation_timestamp": 1529028062,
		  "effective_amount": "20.001",
		  "expiration_height": 2489455,
		  "is_controlling": true,
		  "reposted": 0,
		  "support_amount": "20.0",
		  "take_over_height": 713239,
		  "trending_global": 0.0,
		  "trending_group": 0,
		  "trending_local": 0.0,
		  "trending_mixed": 0.0
		},
		"name": "what",
		"normalized_name": "what",
		"nout": 0,
		"permanent_url": "lbry://what#19b9c243bea0c45175e6a6027911abbad53e983e",
		"protobuf": "000a570a3d2209766964656f2f6d70343230d5169241150022f996fa7cd6a9a1c421937276a3275eb912790bd07ba7aec1fac5fd45431d226b8fb402691e79aeb24b120c53616d75656c20427279616e1a084c42525920696e63420d57686174206973204c4252593f4a3057686174206973204c4252593f20416e20696e74726f64756374696f6e207769746820416c6578205461626172726f6b52312a2f68747470733a2f2f73332e616d617a6f6e6177732e636f6d2f66696c65732e6c6272792e696f2f6c6f676f2e706e6762020801",
		"short_url": "lbry://what#1",
		"timestamp": 1529028062,
		"txid": "555ef2de37698f1c1d36d1d95bdf7b18d51483c76765a2ca63ff45bae5df65e9",
		"type": "claim",
		"value": {
		  "author": "Samuel Bryan",
		  "description": "What is LBRY? An introduction with Alex Tabarrok",
		  "languages": [
			"en"
		  ],
		  "license": "LBRY inc",
		  "source": {
			"media_type": "video/mp4",
			"sd_hash": "d5169241150022f996fa7cd6a9a1c421937276a3275eb912790bd07ba7aec1fac5fd45431d226b8fb402691e79aeb24b"
		  },
		  "stream_type": "video",
		  "thumbnail": {
			"url": "https://s3.amazonaws.com/files.lbry.io/logo.png"
		  },
		  "title": "What is LBRY?"
		},
		"value_type": "stream"
	  }
	},
	"id": 0
}
`

var resolveResponseCouldntFind = `
{
	"jsonrpc": "2.0",
	"result": {
	  "lbry://@whatever#b/whatever#4": {
		"error": {
		  "name": "NOT_FOUND",
		  "text": "Could not find claim at \"lbry://@whatever#b/whatever#4\"."
		}
	  }
	},
	"id": 0
}
`

var responseInternalError = `{
	"jsonrpc": "2.0",
	"error": {
	  "code": -32000,
	  "message": "sqlite query timed out"
	},
	"id": 0
}`

var responseUserInputError = `{
	"jsonrpc": "2.0",
	"error": {
	  "code": -32500,
	  "message": "expected string or bytes-like object"
	},
	"id": 0
}`
