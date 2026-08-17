package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	coretss "github.com/BroLabel/brosettlement-mpc-core/tss"
	"github.com/btcsuite/btcd/btcec"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestDiscoverWalletBySourceAddress(t *testing.T) {
	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), bytes.Repeat([]byte{0x21}, 32))
	defer privateKey.D.SetInt64(0)
	accountPublic := publicKey.SerializeUncompressed()
	chainCode := bytes.Repeat([]byte{0x42}, 32)
	network := networkConfig{name: "nile", chain: nileChain}
	targetIndex := 7
	targetContext := coretss.DerivationContext{
		ProfileID:       "test",
		Chain:           nileChain,
		Algorithm:       coretss.AlgorithmECDSA,
		Curve:           coretss.CurveSecp256k1,
		Scheme:          coretss.DerivationSchemeBIP32Secp256k1,
		PublicKeyFormat: coretss.PublicKeyFormatUncompressedHex,
		AccountPath:     tronAccountPath,
		ChildPath:       fmt.Sprintf("/0/%d", targetIndex),
		FullPath:        fmt.Sprintf("%s/0/%d", tronAccountPath, targetIndex),
		AddressEncoding: "tron_base58check",
	}
	targetPublic, err := coretss.DeriveECDSAChildPublicKey(hex.EncodeToString(accountPublic), chainCode, targetContext)
	if err != nil {
		t.Fatal(err)
	}
	targetAddress, err := tronAddressFromPublicKey(targetPublic)
	if err != nil {
		t.Fatal(err)
	}

	gotContext, gotPublic, gotAddress, gotIndex, err := discoverWallet(accountPublic, chainCode, targetAddress, targetIndex+1, network)
	if err != nil {
		t.Fatal(err)
	}
	if gotIndex != targetIndex || gotAddress != targetAddress || gotPublic != targetPublic || gotContext.ChildPath != "/0/7" || gotContext.FullPath != "m/44'/195'/0'/0/7" {
		t.Fatalf("unexpected discovery result: index=%d address=%s child=%s full=%s", gotIndex, gotAddress, gotContext.ChildPath, gotContext.FullPath)
	}
	if _, _, _, _, err := discoverWallet(accountPublic, chainCode, targetAddress, targetIndex, network); err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected bounded not-found error, got %v", err)
	}
}

func TestParseDecimalAmount(t *testing.T) {
	tests := []struct {
		raw      string
		decimals int
		want     string
		wantErr  bool
	}{
		{raw: "1", decimals: 6, want: "1000000"},
		{raw: "1.25", decimals: 6, want: "1250000"},
		{raw: "0.000001", decimals: 6, want: "1"},
		{raw: "1.0000001", decimals: 6, wantErr: true},
		{raw: "0", decimals: 6, wantErr: true},
		{raw: "-1", decimals: 6, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			got, err := parseDecimalAmount(test.raw, test.decimals, "--amount")
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", got)
				}
				return
			}
			if err != nil || got.String() != test.want {
				t.Fatalf("got %v, %v; want %s", got, err, test.want)
			}
		})
	}
}

func TestOptionalTokenContractSelectsTransferType(t *testing.T) {
	if got := requestedAssetType(options{}); got != "trx" {
		t.Fatalf("omitted token contract selected %q; want trx", got)
	}
	if got := requestedAssetType(options{tokenContract: "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"}); got != "trc20" {
		t.Fatalf("token contract selected %q; want trc20", got)
	}
}

func TestEncodeTRC20TransferParameters(t *testing.T) {
	const destination = "TV3nb5HYFe2xBEmyb3ETe93UGkjAhWyzrs"
	parameter, err := encodeTRC20TransferParameters(destination, big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	const want = "000000000000000000000041d148171f1ceeeb40d668c47d70e7e94e67977559" +
		"0000000000000000000000000000000000000000000000000000000000000064"
	if parameter != want {
		t.Fatalf("unexpected ABI parameters:\n got %s\nwant %s", parameter, want)
	}
}

func TestCreateTRC20TransactionValidatesSigningIntent(t *testing.T) {
	const (
		source      = "TPnBjYQEMo4Yd4866KCzXdi4a169KGd63n"
		destination = "TV3nb5HYFe2xBEmyb3ETe93UGkjAhWyzrs"
		contract    = "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"
		feeLimit    = int64(100_000_000)
	)
	amount := big.NewInt(50)
	parameter, err := encodeTRC20TransferParameters(destination, amount)
	if err != nil {
		t.Fatal(err)
	}
	rawData := map[string]any{
		"expiration": time.Now().Add(time.Minute).UnixMilli(),
		"timestamp":  time.Now().UnixMilli(),
		"fee_limit":  feeLimit,
		"contract": []any{map[string]any{
			"type": "TriggerSmartContract",
			"parameter": map[string]any{"value": map[string]any{
				"owner_address":    source,
				"contract_address": contract,
				"call_value":       0,
				"data":             transferSelector + parameter,
			}},
		}},
	}
	rawJSON, err := json.Marshal(rawData)
	if err != nil {
		t.Fatal(err)
	}
	rawHex := "01020304"
	digest := sha256.Sum256([]byte{1, 2, 3, 4})
	txID := hex.EncodeToString(digest[:])

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/wallet/triggersmartcontract" {
			return nil, fmt.Errorf("unexpected path %s", request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["function_selector"] != "transfer(address,uint256)" || body["parameter"] != parameter || body["owner_address"] != source || body["contract_address"] != contract {
			t.Errorf("unexpected trigger request: %#v", body)
		}
		responseBody, err := json.Marshal(map[string]any{
			"result": map[string]any{"result": true},
			"txid":   txID,
			"transaction": map[string]any{
				"visible":      true,
				"txID":         txID,
				"raw_data":     json.RawMessage(rawJSON),
				"raw_data_hex": rawHex,
			},
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
			Header:     make(http.Header),
		}, nil
	})}

	network := networkConfig{name: "test", chain: nileChain, rpcBase: "https://rpc.invalid"}
	transaction, raw, gotDigest, err := createTRC20Transaction(client, network, nil, source, destination, contract, amount, feeLimit)
	if err != nil {
		t.Fatal(err)
	}
	if transaction.TxID != txID || raw.FeeLimit != feeLimit || !bytes.Equal(gotDigest, digest[:]) {
		t.Fatalf("unexpected transaction validation result: tx=%s fee=%d digest=%x", transaction.TxID, raw.FeeLimit, gotDigest)
	}
}
