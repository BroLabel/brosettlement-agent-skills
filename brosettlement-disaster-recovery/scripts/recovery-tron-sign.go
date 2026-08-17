// recovery-tron-sign creates, threshold-signs, and broadcasts one TRON TRX
// transfer with client-controlled Share B + Share C.
//
// Keep B and C in separate trust domains outside a controlled recovery
// ceremony. This file contains no custody-platform API integration.
package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BroLabel/brosettlement-mpc-core/protocol"
	coretss "github.com/BroLabel/brosettlement-mpc-core/tss"
	"github.com/bnb-chain/tss-lib/common"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/sha3"
	"golang.org/x/sync/errgroup"
)

const (
	nileRPC            = "https://nile.trongrid.io"
	mainnetRPC         = "https://api.trongrid.io"
	nileChain          = "tron:nile"
	mainnetChain       = "tron:mainnet"
	artifactLimit      = 16 << 20
	payloadLimit       = 8 << 20
	rpcResponseLimit   = 1 << 20
	signingTimeout     = 4 * time.Minute
	minimumRPCValidity = 30 * time.Second
	tronAccountPath    = "m/44'/195'/0'"
	tronChildPath      = "/0/0"
)

var canonicalKeyIDPattern = regexp.MustCompile(`^mpc_key_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type options struct {
	network           string
	source            string
	destination       string
	amount            string
	output            string
	shareB            string
	shareC            string
	encryptionKeyPath string
}

type networkConfig struct {
	name    string
	chain   string
	rpcBase string
}

type artifactEnvelope struct {
	Version    int                `json:"version"`
	Encryption artifactEncryption `json:"encryption"`
}

type artifactEncryption struct {
	Algorithm  string `json:"algorithm"`
	KeyRef     string `json:"keyRef"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	Tag        string `json:"tag"`
}

type artifactPayload struct {
	ArtifactPayloadVersion int    `json:"artifactPayloadVersion"`
	SessionID              string `json:"sessionId"`
	PartyID                string `json:"partyId"`
	DescriptorBytesBase64  string `json:"descriptorBytesBase64"`
	ShareBlob              string `json:"shareBlob"`
}

type descriptorParty struct {
	PartyID string `json:"partyId"`
	Purpose string `json:"purpose"`
}

type keyDescriptor struct {
	Algorithm         string            `json:"algorithm"`
	ChainCodeHash     string            `json:"chainCodeHash"`
	Curve             string            `json:"curve"`
	DerivationScheme  string            `json:"derivationScheme"`
	DescriptorKind    string            `json:"descriptorKind"`
	DescriptorVersion int               `json:"descriptorVersion"`
	KeyID             string            `json:"keyId"`
	Parties           []descriptorParty `json:"parties"`
	ProtocolVersion   int               `json:"protocolVersion"`
	PublicKeyFormat   string            `json:"publicKeyFormat"`
	Threshold         int               `json:"threshold"`
}

type loadedArtifact struct {
	sessionID       string
	partyID         string
	keyRef          string
	descriptor      keyDescriptor
	descriptorBytes []byte
	shareBlob       []byte
	accountPublic   []byte
	chainCodeHash   [32]byte
	codecVersion    uint32
}

func (a *loadedArtifact) clear() {
	if a == nil {
		return
	}
	clear(a.descriptorBytes)
	clear(a.shareBlob)
	clear(a.accountPublic)
}

type memoryShareReader struct {
	keyID string
	blob  []byte
}

func (r *memoryShareReader) LoadShare(ctx context.Context, keyID string) (*coretss.StoredShare, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || keyID != r.keyID || len(r.blob) == 0 {
		return nil, coretss.ErrShareNotFound
	}
	return &coretss.StoredShare{Blob: append([]byte(nil), r.blob...)}, nil
}

var _ coretss.ShareReader = (*memoryShareReader)(nil)

type tronAccount struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
}

type unsignedTransaction struct {
	Visible    bool            `json:"visible"`
	TxID       string          `json:"txID"`
	RawData    json.RawMessage `json:"raw_data"`
	RawDataHex string          `json:"raw_data_hex"`
	Error      string          `json:"Error"`
}

type rawTransactionData struct {
	Expiration int64 `json:"expiration"`
	Timestamp  int64 `json:"timestamp"`
	Contracts  []struct {
		Parameter struct {
			Value struct {
				Amount       int64  `json:"amount"`
				OwnerAddress string `json:"owner_address"`
				ToAddress    string `json:"to_address"`
			} `json:"value"`
		} `json:"parameter"`
		Type string `json:"type"`
	} `json:"contract"`
}

type signedTransaction struct {
	Visible    bool            `json:"visible"`
	TxID       string          `json:"txID"`
	RawData    json.RawMessage `json:"raw_data"`
	RawDataHex string          `json:"raw_data_hex"`
	Signature  []string        `json:"signature"`
}

type broadcastResponse struct {
	Result  bool   `json:"result"`
	TxID    string `json:"txid"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cliResult struct {
	Status                string `json:"status"`
	Network               string `json:"network"`
	Chain                 string `json:"chain"`
	SourceAddress         string `json:"sourceAddress"`
	Destination           string `json:"destination,omitempty"`
	AmountSun             int64  `json:"amountSun,omitempty"`
	BalanceBeforeSun      int64  `json:"balanceBeforeSun,omitempty"`
	AccountPath           string `json:"accountPath"`
	ChildPath             string `json:"childPath"`
	DerivedAddressMatch   bool   `json:"derivedAddressMatch"`
	ArtifactsMatch        bool   `json:"artifactsMatch"`
	SignatureVerified     bool   `json:"signatureVerified,omitempty"`
	BroadcastPerformed    bool   `json:"broadcastPerformed"`
	BroadcastAccepted     bool   `json:"broadcastAccepted,omitempty"`
	TxID                  string `json:"txId,omitempty"`
	ExpiresAtUnixMS       int64  `json:"expiresAtUnixMs,omitempty"`
	SignedTransactionFile string `json:"signedTransactionFile,omitempty"`
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("recovery-tron-sign", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&opts.network, "network", "", "required network: nile or mainnet")
	flags.StringVar(&opts.source, "source", "", "required TRON source address on the selected network")
	flags.StringVar(&opts.destination, "to", "", "required TRON destination address on the selected network")
	flags.StringVar(&opts.amount, "amount", "", "required exact TRX amount, up to 6 decimal places")
	flags.StringVar(&opts.output, "output", "", "required absolute path for the signed transaction JSON; must not already exist")
	flags.StringVar(&opts.shareB, "share-b", "", "required absolute path to encrypted Share B artifact")
	flags.StringVar(&opts.shareC, "share-c", "", "required absolute path to encrypted Share C artifact")
	flags.StringVar(&opts.encryptionKeyPath, "encryption-key", "", "required absolute path to the matching share encryption key")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Create, B+C threshold-sign, save, and broadcast a TRON TRX transaction.")
		fmt.Fprintln(flags.Output(), "All eight options are required; key metadata is authenticated from the artifacts.")
		fmt.Fprintln(flags.Output(), "\nUsage:")
		fmt.Fprintln(flags.Output(), "  go run ./recovery-tron-sign.go --network <nile|mainnet> --source <ADDRESS> --to <ADDRESS> --amount <TRX> --output <ABSOLUTE_JSON_PATH> --share-b <ABSOLUTE_PATH> --share-c <ABSOLUTE_PATH> --encryption-key <ABSOLUTE_PATH>")
		fmt.Fprintln(flags.Output(), "\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, errors.New("unexpected positional arguments")
	}
	return opts, nil
}

func run(opts options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	network, err := resolveNetwork(opts.network)
	if err != nil {
		return err
	}
	encryptionKey, err := readEncryptionKey(opts.encryptionKeyPath)
	if err != nil {
		return err
	}
	defer clear(encryptionKey)
	primary, err := loadArtifact(opts.shareB, encryptionKey)
	if err != nil {
		return errors.New("load and authenticate encrypted Share B artifact")
	}
	defer primary.clear()
	recovery, err := loadArtifact(opts.shareC, encryptionKey)
	if err != nil {
		return errors.New("load and authenticate encrypted Share C artifact")
	}
	defer recovery.clear()
	if err := validateArtifactPair(primary, recovery); err != nil {
		return err
	}
	chainCode, err := chainCodeFromShare(primary.shareBlob)
	if err != nil {
		return errors.New("read protected derivation material")
	}
	defer clear(chainCode)
	context, derivedPublicKey, derivedAddress, err := deriveWallet(primary.accountPublic, chainCode, opts, network)
	if err != nil {
		return err
	}
	if derivedAddress != opts.source {
		return errors.New("derived B+C public key does not match the requested source address")
	}
	amountSun, err := parseTRXAmount(opts.amount)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	balance, err := fetchBalance(client, network, nil, opts.source)
	if err != nil {
		return err
	}
	if balance < amountSun {
		return fmt.Errorf("insufficient public on-chain balance: have %d sun, need %d sun", balance, amountSun)
	}
	transaction, raw, digest, err := createTransaction(client, network, nil, opts.source, opts.destination, amountSun)
	if err != nil {
		return err
	}
	defer clear(digest)
	signature, err := signDigest(primary.descriptor.KeyID, context, digest, primary.partyID, recovery.partyID, primary.shareBlob, recovery.shareBlob)
	if err != nil {
		return err
	}
	if err := verifySignature(signature, derivedPublicKey, digest); err != nil {
		return err
	}
	signatureHex, err := encodeTronSignature(signature)
	if err != nil {
		return err
	}
	signed := signedTransaction{
		Visible:    transaction.Visible,
		TxID:       transaction.TxID,
		RawData:    transaction.RawData,
		RawDataHex: transaction.RawDataHex,
		Signature:  []string{signatureHex},
	}
	if err := writeSignedTransaction(opts.output, signed); err != nil {
		return err
	}
	broadcast, err := broadcastTransaction(client, network, nil, signed)
	if err != nil {
		return err
	}
	if !broadcast.Result || (broadcast.TxID != "" && !strings.EqualFold(broadcast.TxID, transaction.TxID)) {
		return fmt.Errorf("%s RPC rejected signed transaction: code=%s message=%s", network.name, broadcast.Code, broadcast.Message)
	}
	return printResult(cliResult{
		Status:                "BROADCAST_ACCEPTED",
		Network:               network.name,
		Chain:                 network.chain,
		SourceAddress:         opts.source,
		Destination:           opts.destination,
		AmountSun:             amountSun,
		BalanceBeforeSun:      balance,
		AccountPath:           tronAccountPath,
		ChildPath:             tronChildPath,
		DerivedAddressMatch:   true,
		ArtifactsMatch:        true,
		SignatureVerified:     true,
		BroadcastPerformed:    true,
		BroadcastAccepted:     true,
		TxID:                  transaction.TxID,
		ExpiresAtUnixMS:       raw.Expiration,
		SignedTransactionFile: opts.output,
	})
}

func validateOptions(opts options) error {
	if _, err := resolveNetwork(opts.network); err != nil {
		return err
	}
	if err := validateTronAddress(opts.source); err != nil {
		return errors.New("invalid source TRON address")
	}
	if opts.shareB == "" || opts.shareC == "" || opts.encryptionKeyPath == "" {
		return errors.New("--share-b, --share-c, and --encryption-key are required")
	}
	for _, path := range []string{opts.shareB, opts.shareC, opts.encryptionKeyPath} {
		if !filepath.IsAbs(path) {
			return errors.New("share and encryption-key paths must be absolute")
		}
	}
	if samePath(opts.shareB, opts.shareC) {
		return errors.New("Share B and Share C paths must be distinct")
	}
	if err := validateTronAddress(opts.destination); err != nil {
		return errors.New("invalid destination TRON address")
	}
	if opts.source == opts.destination {
		return errors.New("source and destination must differ")
	}
	if _, err := parseTRXAmount(opts.amount); err != nil {
		return err
	}
	if !filepath.IsAbs(opts.output) {
		return errors.New("--output must be an absolute path")
	}
	if samePath(opts.output, opts.shareB) || samePath(opts.output, opts.shareC) || samePath(opts.output, opts.encryptionKeyPath) {
		return errors.New("output path conflicts with protected recovery input")
	}
	if _, err := os.Lstat(opts.output); err == nil {
		return errors.New("output file already exists; choose a new path")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("inspect output path")
	}
	return nil
}

func resolveNetwork(raw string) (networkConfig, error) {
	switch raw {
	case "nile":
		return networkConfig{name: "nile", chain: nileChain, rpcBase: nileRPC}, nil
	case "mainnet":
		return networkConfig{name: "mainnet", chain: mainnetChain, rpcBase: mainnetRPC}, nil
	default:
		return networkConfig{}, errors.New("--network is required and must be exactly nile or mainnet")
	}
}

func samePath(first, second string) bool {
	return filepath.Clean(first) == filepath.Clean(second)
}

func readPrivateFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() <= 0 || info.Size() > limit {
		return nil, errors.New("protected input must be a mode-600 regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("open protected input")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("validate protected input")
	}
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(contents) == 0 || int64(len(contents)) > limit {
		clear(contents)
		return nil, errors.New("read protected input")
	}
	return contents, nil
}

func readEncryptionKey(path string) ([]byte, error) {
	encoded, err := readPrivateFile(path, 256)
	if err != nil {
		return nil, errors.New("read protected share encryption key")
	}
	defer clear(encoded)
	trimmed := bytes.TrimSpace(encoded)
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(trimmed)))
	n, err := base64.StdEncoding.Decode(decoded, trimmed)
	if err != nil || n != 32 {
		clear(decoded)
		return nil, errors.New("share encryption key must be canonical base64 for exactly 32 bytes")
	}
	decoded = decoded[:n]
	reencoded := make([]byte, base64.StdEncoding.EncodedLen(n))
	base64.StdEncoding.Encode(reencoded, decoded)
	defer clear(reencoded)
	if !bytes.Equal(reencoded, trimmed) {
		clear(decoded)
		return nil, errors.New("share encryption key is not canonical base64")
	}
	return decoded, nil
}

func loadArtifact(path string, encryptionKey []byte) (*loadedArtifact, error) {
	artifactBytes, err := readPrivateFile(path, artifactLimit)
	if err != nil {
		return nil, err
	}
	defer clear(artifactBytes)
	var envelope artifactEnvelope
	if err := decodeCanonicalJSON(artifactBytes, &envelope); err != nil {
		return nil, errors.New("decode canonical artifact envelope")
	}
	if envelope.Version != 1 || envelope.Encryption.Algorithm != "AES-256-GCM" || !validKeyRef(envelope.Encryption.KeyRef) {
		return nil, errors.New("artifact encryption binding mismatch")
	}
	nonce, err := decodeCanonicalBase64(envelope.Encryption.Nonce, 12, 12)
	if err != nil {
		return nil, errors.New("decode artifact nonce")
	}
	defer clear(nonce)
	ciphertext, err := decodeCanonicalBase64(envelope.Encryption.Ciphertext, 1, 12<<20)
	if err != nil {
		return nil, errors.New("decode artifact ciphertext")
	}
	defer clear(ciphertext)
	tag, err := decodeCanonicalBase64(envelope.Encryption.Tag, 16, 16)
	if err != nil {
		return nil, errors.New("decode artifact authentication tag")
	}
	defer clear(tag)
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, errors.New("initialize artifact cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize artifact authentication")
	}
	sealed := append(append(make([]byte, 0, len(ciphertext)+len(tag)), ciphertext...), tag...)
	defer clear(sealed)
	payloadBytes, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, errors.New("authenticate and decrypt artifact")
	}
	defer clear(payloadBytes)
	if len(payloadBytes) == 0 || len(payloadBytes) > payloadLimit {
		return nil, errors.New("decrypted artifact payload exceeds limit")
	}
	var payload artifactPayload
	if err := decodeCanonicalJSON(payloadBytes, &payload); err != nil {
		return nil, errors.New("decode canonical artifact payload")
	}
	if payload.ArtifactPayloadVersion != 1 || payload.SessionID == "" || payload.PartyID == "" {
		return nil, errors.New("invalid artifact payload contract")
	}
	descriptorBytes, err := decodeCanonicalBase64(payload.DescriptorBytesBase64, 1, 2048)
	if err != nil {
		return nil, errors.New("decode artifact descriptor")
	}
	descriptor, err := parseKeyDescriptor(descriptorBytes)
	if err != nil {
		clear(descriptorBytes)
		return nil, errors.New("validate canonical MPC descriptor")
	}
	shareBlob, err := decodeCanonicalBase64(payload.ShareBlob, 1, payloadLimit)
	if err != nil {
		clear(descriptorBytes)
		return nil, errors.New("decode protected MPC share")
	}
	inspected, err := coretss.InspectEncodedECDSAKeyMaterial(shareBlob)
	if err != nil {
		clear(descriptorBytes)
		clear(shareBlob)
		return nil, errors.New("inspect protected MPC share")
	}
	return &loadedArtifact{
		sessionID:       payload.SessionID,
		partyID:         payload.PartyID,
		keyRef:          envelope.Encryption.KeyRef,
		descriptor:      descriptor,
		descriptorBytes: descriptorBytes,
		shareBlob:       shareBlob,
		accountPublic:   append([]byte(nil), inspected.AccountPublicKey...),
		chainCodeHash:   inspected.ChainCodeHash,
		codecVersion:    inspected.CodecVersion,
	}, nil
}

func decodeCanonicalBase64(raw string, minimum, maximum int) ([]byte, error) {
	if raw == "" || len(raw) > base64.StdEncoding.EncodedLen(maximum) {
		return nil, errors.New("base64 input size")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(decoded) < minimum || len(decoded) > maximum || base64.StdEncoding.EncodeToString(decoded) != raw {
		clear(decoded)
		return nil, errors.New("non-canonical base64")
	}
	return decoded, nil
}

func validKeyRef(raw string) bool {
	if raw == "" || len(raw) > 255 || strings.TrimSpace(raw) != raw {
		return false
	}
	for _, value := range []byte(raw) {
		if value < 0x21 || value > 0x7e {
			return false
		}
	}
	return true
}

func decodeCanonicalJSON(raw []byte, target any) error {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	canonical, err := json.Marshal(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonical) {
		return errors.New("JSON is not in canonical compact field representation")
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return errors.New("JSON object name is not a string")
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("duplicate JSON field %q", name)
			}
			seen[name] = struct{}{}
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func parseKeyDescriptor(raw []byte) (keyDescriptor, error) {
	var descriptor keyDescriptor
	if len(raw) == 0 || len(raw) > 2048 {
		return descriptor, errors.New("descriptor requires bounded bytes")
	}
	for _, value := range raw {
		if value < 0x20 || value > 0x7e {
			return descriptor, errors.New("descriptor must contain printable ASCII only")
		}
	}
	if err := decodeCanonicalJSON(raw, &descriptor); err != nil {
		return descriptor, err
	}
	if descriptor.DescriptorKind != "mpc-key-descriptor" || descriptor.DescriptorVersion != 1 || descriptor.ProtocolVersion != 1 ||
		descriptor.Algorithm != "ECDSA" || descriptor.Curve != "secp256k1" || descriptor.Threshold != 2 ||
		descriptor.DerivationScheme != "bip32_secp256k1" || descriptor.PublicKeyFormat != "compressed_sec1" ||
		!canonicalKeyIDPattern.MatchString(descriptor.KeyID) {
		return keyDescriptor{}, errors.New("unsupported key descriptor contract")
	}
	if err := validateChainCodeHash(descriptor.ChainCodeHash); err != nil {
		return keyDescriptor{}, err
	}
	if len(descriptor.Parties) != 3 {
		return keyDescriptor{}, errors.New("descriptor must contain exactly three parties")
	}
	seenIDs := make(map[string]struct{}, 3)
	seenPurposes := make(map[string]struct{}, 3)
	for index, party := range descriptor.Parties {
		if party.PartyID == "" || len(party.PartyID) > 255 || strings.TrimSpace(party.PartyID) != party.PartyID {
			return keyDescriptor{}, fmt.Errorf("invalid descriptor party at index %d", index)
		}
		for _, value := range []byte(party.PartyID) {
			if value < 0x21 || value > 0x7e {
				return keyDescriptor{}, fmt.Errorf("invalid descriptor party at index %d", index)
			}
		}
		if party.Purpose != "platform" && party.Purpose != "primary" && party.Purpose != "recovery" {
			return keyDescriptor{}, fmt.Errorf("invalid descriptor purpose at index %d", index)
		}
		if _, exists := seenIDs[party.PartyID]; exists {
			return keyDescriptor{}, errors.New("descriptor party IDs must be unique")
		}
		if _, exists := seenPurposes[party.Purpose]; exists {
			return keyDescriptor{}, errors.New("descriptor purposes must be unique")
		}
		seenIDs[party.PartyID] = struct{}{}
		seenPurposes[party.Purpose] = struct{}{}
	}
	return descriptor, nil
}

func descriptorPartyID(descriptor keyDescriptor, purpose string) (string, bool) {
	for _, party := range descriptor.Parties {
		if party.Purpose == purpose {
			return party.PartyID, true
		}
	}
	return "", false
}

func validateChainCodeHash(raw string) error {
	if len(raw) != 43 {
		return errors.New("invalid descriptor chain-code hash")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != sha256.Size || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		clear(decoded)
		return errors.New("invalid descriptor chain-code hash")
	}
	clear(decoded)
	return nil
}

func chainCodeHashFor(raw []byte) string {
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func validateArtifactPair(primary, recovery *loadedArtifact) error {
	if primary == nil || recovery == nil {
		return errors.New("Share B and Share C are required")
	}
	primaryParty, primaryOK := descriptorPartyID(primary.descriptor, "primary")
	recoveryParty, recoveryOK := descriptorPartyID(primary.descriptor, "recovery")
	if !primaryOK || !recoveryOK || primary.partyID != primaryParty || recovery.partyID != recoveryParty ||
		primary.sessionID != recovery.sessionID || primary.keyRef != recovery.keyRef ||
		!bytes.Equal(primary.descriptorBytes, recovery.descriptorBytes) || !bytes.Equal(primary.accountPublic, recovery.accountPublic) ||
		primary.chainCodeHash != recovery.chainCodeHash || primary.codecVersion != recovery.codecVersion ||
		bytes.Equal(primary.shareBlob, recovery.shareBlob) {
		return errors.New("Share B and Share C do not form one validated recovery quorum")
	}
	chainCode, err := chainCodeFromShare(primary.shareBlob)
	if err != nil {
		return errors.New("read Share B derivation evidence")
	}
	defer clear(chainCode)
	if chainCodeHashFor(chainCode) != primary.descriptor.ChainCodeHash {
		return errors.New("artifact descriptor chain-code binding mismatch")
	}
	return nil
}

func chainCodeFromShare(blob []byte) ([]byte, error) {
	material, err := coretss.UnmarshalKeyMaterial(blob)
	if err != nil {
		return nil, err
	}
	defer clear(material.ChainCode)
	if len(material.ChainCode) != 32 {
		return nil, errors.New("invalid chain code")
	}
	return append([]byte(nil), material.ChainCode...), nil
}

func deriveWallet(accountPublic, chainCode []byte, opts options, network networkConfig) (coretss.DerivationContext, string, string, error) {
	point, err := btcec.ParsePubKey(accountPublic, btcec.S256())
	if err != nil {
		return coretss.DerivationContext{}, "", "", errors.New("parse account public key")
	}
	derivation := coretss.DerivationContext{
		ProfileID:       "tron-recovery-cli-" + network.name,
		Chain:           network.chain,
		Algorithm:       coretss.AlgorithmECDSA,
		Curve:           coretss.CurveSecp256k1,
		Scheme:          coretss.DerivationSchemeBIP32Secp256k1,
		PublicKeyFormat: coretss.PublicKeyFormatUncompressedHex,
		AccountPath:     tronAccountPath,
		ChildPath:       tronChildPath,
		FullPath:        strings.TrimSuffix(tronAccountPath, "/") + tronChildPath,
		AddressEncoding: "tron_base58check",
		ExpectedAddress: opts.source,
	}
	derivedPublicKey, err := coretss.DeriveECDSAChildPublicKey(hex.EncodeToString(point.SerializeUncompressed()), chainCode, derivation)
	if err != nil {
		return coretss.DerivationContext{}, "", "", errors.New("derive TRON child public key")
	}
	derivedAddress, err := tronAddressFromPublicKey(derivedPublicKey)
	if err != nil {
		return coretss.DerivationContext{}, "", "", err
	}
	derivation.DerivedPublicKey = derivedPublicKey
	return derivation, derivedPublicKey, derivedAddress, nil
}

func validateTronAddress(address string) error {
	payload, version, err := base58.CheckDecode(address)
	if err != nil || version != 0x41 || len(payload) != 20 {
		return errors.New("invalid TRON Base58Check address")
	}
	return nil
}

func tronAddressFromPublicKey(publicKeyHex string) (string, error) {
	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(publicKey) != 65 || publicKey[0] != 0x04 {
		return "", errors.New("invalid uncompressed secp256k1 public key")
	}
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(publicKey[1:])
	digest := hasher.Sum(nil)
	defer clear(digest)
	return base58.CheckEncode(digest[len(digest)-20:], 0x41), nil
}

func parseTRXAmount(raw string) (int64, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		return 0, errors.New("--amount must be a positive decimal with up to 6 fractional digits")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return 0, errors.New("--amount must be a positive decimal with up to 6 fractional digits")
	}
	if !decimalDigits(parts[0]) || (len(parts) == 2 && (!decimalDigits(parts[1]) || len(parts[1]) > 6)) {
		return 0, errors.New("--amount must be a positive decimal with up to 6 fractional digits")
	}
	whole, err := strconv.ParseUint(parts[0], 10, 63)
	if err != nil || whole > uint64((int64(^uint64(0)>>1))/1_000_000) {
		return 0, errors.New("--amount is too large")
	}
	fraction := uint64(0)
	if len(parts) == 2 {
		padded := parts[1] + strings.Repeat("0", 6-len(parts[1]))
		fraction, err = strconv.ParseUint(padded, 10, 32)
		if err != nil {
			return 0, errors.New("invalid fractional TRX amount")
		}
	}
	amount := int64(whole*1_000_000 + fraction)
	if amount <= 0 {
		return 0, errors.New("--amount must be greater than zero")
	}
	return amount, nil
}

func decimalDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func fetchBalance(client *http.Client, network networkConfig, apiKey []byte, address string) (int64, error) {
	body, _ := json.Marshal(map[string]any{"address": address, "visible": true})
	defer clear(body)
	response, err := postRPCJSON(client, network, apiKey, "/wallet/getaccount", body)
	if err != nil {
		return 0, fmt.Errorf("query source balance through %s RPC", network.name)
	}
	defer clear(response)
	var account tronAccount
	if err := json.Unmarshal(response, &account); err != nil || account.Address != address {
		return 0, fmt.Errorf("validate %s account response", network.name)
	}
	return account.Balance, nil
}

func createTransaction(client *http.Client, network networkConfig, apiKey []byte, source, destination string, amountSun int64) (unsignedTransaction, rawTransactionData, []byte, error) {
	body, _ := json.Marshal(map[string]any{
		"owner_address": source,
		"to_address":    destination,
		"amount":        amountSun,
		"visible":       true,
	})
	defer clear(body)
	response, err := postRPCJSON(client, network, apiKey, "/wallet/createtransaction", body)
	if err != nil {
		return unsignedTransaction{}, rawTransactionData{}, nil, fmt.Errorf("create unsigned transaction through %s RPC", network.name)
	}
	defer clear(response)
	var transaction unsignedTransaction
	if err := json.Unmarshal(response, &transaction); err != nil || transaction.Error != "" || transaction.TxID == "" || transaction.RawDataHex == "" {
		return unsignedTransaction{}, rawTransactionData{}, nil, errors.New("validate unsigned transaction response")
	}
	rawBytes, err := hex.DecodeString(transaction.RawDataHex)
	if err != nil {
		return unsignedTransaction{}, rawTransactionData{}, nil, errors.New("decode unsigned transaction raw data")
	}
	hash := sha256.Sum256(rawBytes)
	clear(rawBytes)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), transaction.TxID) {
		return unsignedTransaction{}, rawTransactionData{}, nil, errors.New("transaction ID does not match SHA-256(raw_data)")
	}
	var raw rawTransactionData
	if err := json.Unmarshal(transaction.RawData, &raw); err != nil || len(raw.Contracts) != 1 || raw.Contracts[0].Type != "TransferContract" ||
		raw.Contracts[0].Parameter.Value.OwnerAddress != source || raw.Contracts[0].Parameter.Value.ToAddress != destination ||
		raw.Contracts[0].Parameter.Value.Amount != amountSun || raw.Expiration < time.Now().Add(minimumRPCValidity).UnixMilli() {
		return unsignedTransaction{}, rawTransactionData{}, nil, errors.New("unsigned transaction does not match the requested signing intent")
	}
	return transaction, raw, append([]byte(nil), hash[:]...), nil
}

func postRPCJSON(client *http.Client, network networkConfig, apiKey []byte, path string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, network.rpcBase+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if len(apiKey) != 0 {
		request.Header.Set("TRON-PRO-API-KEY", string(apiKey))
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(response.Body, rpcResponseLimit+1))
	if err != nil || len(contents) == 0 || len(contents) > rpcResponseLimit {
		clear(contents)
		return nil, fmt.Errorf("read bounded %s RPC response", network.name)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		clear(contents)
		return nil, fmt.Errorf("%s RPC returned HTTP %d", network.name, response.StatusCode)
	}
	return contents, nil
}

func broadcastTransaction(client *http.Client, network networkConfig, apiKey []byte, transaction signedTransaction) (broadcastResponse, error) {
	body, err := json.Marshal(transaction)
	if err != nil {
		return broadcastResponse{}, errors.New("encode signed transaction for broadcast")
	}
	defer clear(body)
	response, err := postRPCJSON(client, network, apiKey, "/wallet/broadcasttransaction", body)
	if err != nil {
		return broadcastResponse{}, fmt.Errorf("broadcast signed transaction through %s RPC", network.name)
	}
	defer clear(response)
	var result broadcastResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return broadcastResponse{}, fmt.Errorf("decode %s broadcast response", network.name)
	}
	return result, nil
}

func signDigest(keyID string, derivation coretss.DerivationContext, digest []byte, primaryPartyID, recoveryPartyID string, primaryBlob, recoveryBlob []byte) (*common.SignatureData, error) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	primaryService := coretss.NewBnbService(logger, coretss.WithShareReader(&memoryShareReader{keyID: keyID, blob: primaryBlob}))
	recoveryService := coretss.NewBnbService(logger, coretss.WithShareReader(&memoryShareReader{keyID: keyID, blob: recoveryBlob}))
	sessionID := "recovery-tron-cli-sign-" + hex.EncodeToString(digest[:8])
	parties := []string{primaryPartyID, recoveryPartyID}
	transports := newFrameBus(parties)
	ctx, cancel := context.WithTimeout(context.Background(), signingTimeout)
	defer cancel()
	group, groupContext := errgroup.WithContext(ctx)
	for _, participant := range []struct {
		partyID string
		service *coretss.Service
	}{
		{partyID: primaryPartyID, service: primaryService},
		{partyID: recoveryPartyID, service: recoveryService},
	} {
		participant := participant
		group.Go(func() error {
			return participant.service.RunSignSession(groupContext, coretss.SignSessionRequest{
				Session: coretss.SignSessionDescriptor{
					SessionID: sessionID,
					OrgID:     "recovery-cli",
					KeyID:     keyID,
					Parties:   parties,
					Threshold: 2,
					Algorithm: coretss.AlgorithmECDSA,
					Curve:     coretss.CurveSecp256k1,
					Chain:     derivation.Chain,
				},
				LocalPartyID:      participant.partyID,
				Digest:            digest,
				DerivationContext: &derivation,
				Transport:         transports[participant.partyID],
			})
		})
	}
	if err := group.Wait(); err != nil {
		return nil, errors.New("B+C MPC signing session failed")
	}
	signature, err := primaryService.ExportECDSASignature(sessionID)
	if err != nil {
		return nil, errors.New("export B+C MPC signature")
	}
	return &signature, nil
}

func verifySignature(signature *common.SignatureData, publicKeyHex string, digest []byte) error {
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return errors.New("decode derived public key")
	}
	publicKey, err := btcec.ParsePubKey(publicKeyBytes, btcec.S256())
	if err != nil {
		return errors.New("parse derived public key")
	}
	r := new(big.Int).SetBytes(signature.GetR())
	s := new(big.Int).SetBytes(signature.GetS())
	recovery := signature.GetSignatureRecovery()
	if len(signature.GetR()) != 32 || len(signature.GetS()) != 32 || len(recovery) != 1 || recovery[0] > 3 ||
		!bytes.Equal(signature.GetM(), digest) || !ecdsa.Verify(publicKey.ToECDSA(), digest, r, s) {
		return errors.New("independent ECDSA verification rejected B+C signature")
	}
	compact := make([]byte, 65)
	compact[0] = 27 + 4 + recovery[0]
	copy(compact[1:33], signature.GetR())
	copy(compact[33:], signature.GetS())
	recovered, _, err := btcec.RecoverCompact(btcec.S256(), compact, digest)
	clear(compact)
	if err != nil || !bytes.Equal(recovered.SerializeUncompressed(), publicKey.SerializeUncompressed()) {
		return errors.New("signature recovery ID does not match derived TRON public key")
	}
	return nil
}

func encodeTronSignature(signature *common.SignatureData) (string, error) {
	recovery := signature.GetSignatureRecovery()
	if len(signature.GetR()) != 32 || len(signature.GetS()) != 32 || len(recovery) != 1 || recovery[0] > 3 {
		return "", errors.New("invalid TRON signature components")
	}
	encoded := make([]byte, 0, 65)
	encoded = append(encoded, signature.GetR()...)
	encoded = append(encoded, signature.GetS()...)
	encoded = append(encoded, recovery[0])
	return hex.EncodeToString(encoded), nil
}

func writeSignedTransaction(path string, transaction signedTransaction) error {
	encoded, err := json.MarshalIndent(transaction, "", "  ")
	if err != nil {
		return errors.New("encode signed transaction")
	}
	encoded = append(encoded, '\n')
	defer clear(encoded)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("create new signed transaction file")
	}
	written := false
	defer func() {
		_ = file.Close()
		if !written {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		return errors.New("write signed transaction file")
	}
	if err := file.Sync(); err != nil {
		return errors.New("sync signed transaction file")
	}
	if err := file.Close(); err != nil {
		return errors.New("close signed transaction file")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		return errors.New("validate signed transaction file permissions")
	}
	written = true
	return nil
}

func printResult(result cliResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

type frameBus struct {
	mu        sync.RWMutex
	endpoints map[string]chan protocol.Frame
}

type frameTransport struct {
	bus     *frameBus
	inbound <-chan protocol.Frame
}

func newFrameBus(parties []string) map[string]coretss.Transport {
	bus := &frameBus{endpoints: make(map[string]chan protocol.Frame, len(parties))}
	transports := make(map[string]coretss.Transport, len(parties))
	for _, partyID := range parties {
		inbound := make(chan protocol.Frame, 256)
		bus.endpoints[partyID] = inbound
		transports[partyID] = &frameTransport{bus: bus, inbound: inbound}
	}
	return transports
}

func (t *frameTransport) SendFrame(ctx context.Context, frame protocol.Frame) error {
	t.bus.mu.RLock()
	defer t.bus.mu.RUnlock()
	if frame.IsBroadcast() {
		for partyID, inbound := range t.bus.endpoints {
			if partyID == frame.FromParty {
				continue
			}
			if err := sendFrame(ctx, inbound, frame); err != nil {
				return err
			}
		}
		return nil
	}
	inbound, ok := t.bus.endpoints[frame.ToParty]
	if !ok {
		return errors.New("recovery transport target unavailable")
	}
	return sendFrame(ctx, inbound, frame)
}

func (t *frameTransport) RecvFrame(ctx context.Context) (protocol.Frame, error) {
	select {
	case frame := <-t.inbound:
		return frame, nil
	case <-ctx.Done():
		return protocol.Frame{}, ctx.Err()
	}
}

func sendFrame(ctx context.Context, inbound chan<- protocol.Frame, frame protocol.Frame) error {
	copyFrame := frame
	copyFrame.Payload = append([]byte(nil), frame.Payload...)
	select {
	case inbound <- copyFrame:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
