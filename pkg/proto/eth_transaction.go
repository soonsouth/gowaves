package proto

import (
	"fmt"
	"io"
	"math/big"

	"github.com/pkg/errors"
	"github.com/umbracle/fastrlp"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/errs"
	g "github.com/wavesplatform/gowaves/pkg/grpc/generated/waves"
	"go.uber.org/atomic"
)

// EthereumGasPrice is a constant GasPrice which equals 10GWei according to the specification
const EthereumGasPrice = 10 * ethereumGWei

// EthereumTxType is an ethereum transaction type.
type EthereumTxType byte

const (
	EthereumLegacyTxType EthereumTxType = iota
	EthereumAccessListTxType
	EthereumDynamicFeeTxType
)

func (e EthereumTxType) String() string {
	switch e {
	case EthereumLegacyTxType:
		return "EthereumLegacyTxType"
	case EthereumAccessListTxType:
		return "EthereumAccessListTxType"
	case EthereumDynamicFeeTxType:
		return "EthereumDynamicFeeTxType"
	default:
		return ""
	}
}

var (
	ErrInvalidSig         = errors.New("invalid transaction v, r, s values")
	ErrTxTypeNotSupported = errors.New("transaction type not supported")
)

type fastRLPSignerHasher interface {
	signerHashFastRLP(chainID *big.Int, arena *fastrlp.Arena) *fastrlp.Value
}

type RLPDecoder interface {
	DecodeRLP([]byte) error
}

type RLPEncoder interface {
	EncodeRLP(io.Writer) error
}

type fastRLPMarshaler interface {
	marshalToFastRLP(arena *fastrlp.Arena) *fastrlp.Value
}

type EthereumTxData interface {
	ethereumTxType() EthereumTxType
	copy() EthereumTxData // creates a deep copy and initializes all fields

	chainID() *big.Int
	accessList() EthereumAccessList
	data() []byte
	gas() uint64
	gasPrice() *big.Int
	gasTipCap() *big.Int
	gasFeeCap() *big.Int
	value() *big.Int
	nonce() uint64
	to() *EthereumAddress

	rawSignatureValues() (v, r, s *big.Int)
	setSignatureValues(chainID, v, r, s *big.Int)

	fastRLPMarshaler
	fastRLPSignerHasher
}

type EthereumTransaction struct {
	inner           EthereumTxData
	innerBinarySize int
	id              *crypto.Digest
	senderPK        atomic.Value // *EthereumPublicKey
}

func (tx *EthereumTransaction) GetTypeInfo() TransactionTypeInfo {
	return TransactionTypeInfo{
		Type:         EthereumMetamaskTransaction,
		ProofVersion: Proof,
	}
}

func (tx *EthereumTransaction) GetVersion() byte {
	// EthereumTransaction version always should be zero
	return 0
}

func (tx *EthereumTransaction) GetID(scheme Scheme) ([]byte, error) {
	if tx.id == nil {
		if err := tx.GenerateID(scheme); err != nil {
			return nil, err
		}
	}
	return tx.id.Bytes(), nil
}

func (tx *EthereumTransaction) GetSender(_ Scheme) (Address, error) {
	return tx.From()
}

func (tx *EthereumTransaction) GetFee() uint64 {
	// in scala node this is "gasLimit" field.
	return tx.Gas()
}

func (tx *EthereumTransaction) GetTimestamp() uint64 {
	return tx.Nonce()
}

func (tx *EthereumTransaction) threadSafeGetSenderPK() *EthereumPublicKey {
	senderPK := tx.senderPK.Load()
	if senderPK != nil {
		return senderPK.(*EthereumPublicKey)
	}
	return nil
}

func (tx *EthereumTransaction) threadSafeSetSenderPK(senderPK *EthereumPublicKey) {
	tx.senderPK.Store(senderPK)
}

// Verify performs ONLY transaction signature verification and calculates EthereumPublicKey of transaction
// For basic transaction checks use Validate method
func (tx *EthereumTransaction) Verify() (*EthereumPublicKey, error) {
	if senderPK := tx.threadSafeGetSenderPK(); senderPK != nil {
		return senderPK, nil
	}
	signer := MakeEthereumSigner(tx.ChainId())
	senderPK, err := signer.SenderPK(tx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to verify EthereumTransaction")
	}
	tx.threadSafeSetSenderPK(senderPK)
	return senderPK, nil
}

// Validate performs basic checks for EthereumTransaction according to the specification
// This method doesn't include signature verification. Use Verify method for signature verification
func (tx *EthereumTransaction) Validate(scheme Scheme) (Transaction, error) {
	// same chainID
	if tx.ChainId().Cmp(big.NewInt(int64(scheme))) != 0 {
		// TODO: introduce new error type for scheme validation
		txChainID := tx.ChainId().Uint64()
		return nil, errs.NewTxValidationError(fmt.Sprintf(
			"Address belongs to another network: expected: %d(%c), actual: %d(%c)",
			scheme, scheme,
			txChainID, txChainID,
		))
	}
	// accept only EthereumLegacyTxType (this check doesn't exist in scala)
	if tx.EthereumTxType() != EthereumLegacyTxType {
		return nil, errs.NewTxValidationError("the ethereum transaction's type is not legacy tx")
	}
	// max size of EthereumTransaction is 1Mb (this check doesn't exist in scala)
	if tx.innerBinarySize > 1024*1024 {
		return nil, errs.NewTxValidationError("too big size of transaction")
	}
	// insufficient fee
	if tx.Gas() <= 0 {
		return nil, errs.NewFeeValidation("insufficient fee")
	}
	// too many waves (this check doesn't exist in scala)
	wavelets, err := EthereumWeiToWavelet(tx.Value())
	if err != nil {
		return nil, errs.NewFeeValidation(err.Error())
	}
	// non positive amount
	if wavelets < 0 {
		return nil, errs.NewNonPositiveAmount(wavelets, "waves")
	}
	// a cancel transaction: value == 0 && data == 0x
	if tx.Value().Cmp(big0) == 0 && len(tx.Data()) == 0 {
		return nil, errs.NewTxValidationError("Transaction cancellation is not supported")
	}
	// either data or value field is set
	if tx.Value().Cmp(big0) != 0 && len(tx.Data()) != 0 {
		return nil, errs.NewTxValidationError("Transaction should have either data or value")
	}
	// gasPrice == 10GWei
	if tx.GasPrice().Cmp(new(big.Int).SetUint64(EthereumGasPrice)) != 0 {
		return nil, errs.NewTxValidationError("Gas price must be 10 Gwei")
	}
	// deny a contract creation transaction (this check doesn't exist in scala)
	if tx.To() == nil {
		return nil, errs.NewTxValidationError("Contract creation transaction is not supported")
	}
	// positive timestamp (this check doesn't exist in scala)
	if tx.Nonce() <= 0 {
		return nil, errs.NewTxValidationError("invalid timestamp")
	}
	return tx, nil
}

func (tx *EthereumTransaction) GenerateID(_ Scheme) error {
	if tx.id != nil {
		return nil
	}
	body, err := tx.EncodeCanonical()
	if err != nil {
		return err
	}
	id := Keccak256EthereumHash(body)
	tx.id = (*crypto.Digest)(&id)
	return nil
}

func (tx *EthereumTransaction) MerkleBytes(_ Scheme) ([]byte, error) {
	return tx.EncodeCanonical()
}

func (tx *EthereumTransaction) Sign(_ Scheme, _ crypto.SecretKey) error {
	return errors.New("Sign method for EthereumTransaction isn't implemented")
}

func (tx *EthereumTransaction) MarshalBinary() ([]byte, error) {
	return nil, errors.New("EthereumTransaction does not support 'MarshalBinary' method.")
}

func (tx *EthereumTransaction) UnmarshalBinary(_ []byte, _ Scheme) error {
	return errors.New("EthereumTransaction does not support 'UnmarshalBinary' method.")
}

func (tx *EthereumTransaction) BodyMarshalBinary() ([]byte, error) {
	return nil, errors.New("EthereumTransaction does not support 'BodyMarshalBinary' method.")
}

func (tx *EthereumTransaction) BinarySize() int {
	return 0
}

func (tx *EthereumTransaction) MarshalToProtobuf(_ Scheme) ([]byte, error) {
	return nil, errors.New("EthereumTransaction does not support 'MarshalToProtobuf' method.")
}

func (tx *EthereumTransaction) UnmarshalFromProtobuf(_ []byte) error {
	return errors.New("EthereumTransaction does not support 'UnmarshalFromProtobuf' method.")
}

func (tx *EthereumTransaction) MarshalSignedToProtobuf(scheme Scheme) ([]byte, error) {
	return MarshalSignedTxDeterministic(tx, scheme)
}

func (tx *EthereumTransaction) UnmarshalSignedFromProtobuf(bytes []byte) error {
	t, err := SignedTxFromProtobuf(bytes)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal from protobuf ethereum transaction")
	}
	ethTx, ok := t.(*EthereumTransaction)
	if !ok {
		return errors.Errorf(
			"failed to cast unmarshalled result '%T' to '*EthereumTransaction' type",
			t,
		)
	}
	*tx = *ethTx
	return nil
}

func (tx *EthereumTransaction) ToProtobuf(_ Scheme) (*g.Transaction, error) {
	return nil, errors.New("EthereumTransaction does not support 'ToProtobuf' method.")
}

func (tx *EthereumTransaction) ToProtobufSigned(_ Scheme) (*g.SignedTransaction, error) {
	canonical, err := tx.EncodeCanonical()
	if err != nil {
		return nil, errors.Wrapf(err,
			"failed to marshal binary EthereumTransaction, type %q",
			tx.EthereumTxType().String(),
		)
	}
	signed := g.SignedTransaction{
		Transaction: &g.SignedTransaction_EthereumTransaction{
			EthereumTransaction: canonical,
		},
	}
	return &signed, nil
}

// EthereumTxType returns the transaction type.
func (tx *EthereumTransaction) EthereumTxType() EthereumTxType {
	return tx.inner.ethereumTxType()
}

// ChainId returns the EIP155 chain ID of the transaction. The return value will always be
// non-nil. For legacy transactions which are not replay-protected, the return value is
// zero.
func (tx *EthereumTransaction) ChainId() *big.Int {
	return tx.inner.chainID()
}

// Data returns the input data of the transaction.
func (tx *EthereumTransaction) Data() []byte { return tx.inner.data() }

// AccessList returns the access list of the transaction.
func (tx *EthereumTransaction) AccessList() EthereumAccessList { return tx.inner.accessList() }

// Gas returns the gas limit of the transaction.
func (tx *EthereumTransaction) Gas() uint64 { return tx.inner.gas() }

// GasPrice returns the gas price of the transaction.
func (tx *EthereumTransaction) GasPrice() *big.Int { return copyBigInt(tx.inner.gasPrice()) }

// GasTipCap returns the gasTipCap per gas of the transaction.
func (tx *EthereumTransaction) GasTipCap() *big.Int { return copyBigInt(tx.inner.gasTipCap()) }

// GasFeeCap returns the fee cap per gas of the transaction.
func (tx *EthereumTransaction) GasFeeCap() *big.Int { return copyBigInt(tx.inner.gasFeeCap()) }

// Value returns the ether amount of the transaction.
func (tx *EthereumTransaction) Value() *big.Int { return copyBigInt(tx.inner.value()) }

// Nonce returns the sender account nonce of the transaction.
func (tx *EthereumTransaction) Nonce() uint64 { return tx.inner.nonce() }

// To returns the recipient address of the transaction.
// For contract-creation transactions, To returns nil.
func (tx *EthereumTransaction) To() *EthereumAddress { return tx.inner.to().copy() }

// From returns the sender address of the transaction.
// Returns error if transaction doesn't pass validation.
func (tx *EthereumTransaction) From() (EthereumAddress, error) {
	senderPK, err := tx.Verify()
	if err != nil {
		return EthereumAddress{}, err
	}
	return senderPK.EthereumAddress(), nil
}

// FromPK returns the sender public key of the transaction.
// Returns error if transaction doesn't pass validation.
func (tx *EthereumTransaction) FromPK() (*EthereumPublicKey, error) {
	senderPK, err := tx.Verify()
	if err != nil {
		return nil, err
	}
	return senderPK.copy(), nil
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *EthereumTransaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.inner.rawSignatureValues()
}

// DecodeCanonical decodes the canonical binary encoding of transactions.
// It supports legacy RLP transactions and EIP2718 typed transactions.
func (tx *EthereumTransaction) DecodeCanonical(canonicalData []byte) error {
	// check according to the EIP2718
	if len(canonicalData) > 0 && canonicalData[0] > 0x7f {
		// It's a legacy transaction.
		parser := fastrlp.Parser{}
		value, err := parser.Parse(canonicalData)
		if err != nil {
			return errors.Wrap(err, "failed to parse canonical representation as RLP")
		}
		var inner EthereumLegacyTx
		if err := inner.unmarshalFromFastRLP(value); err != nil {
			return errors.Wrapf(err,
				"failed to unmarshal from RLP ethereum legacy transaction, ethereumTxType %q",
				EthereumLegacyTxType.String(),
			)
		}
		tx.inner = &inner
	} else {
		// It's an EIP2718 typed transaction envelope.
		inner, err := tx.decodeTypedCanonical(canonicalData)
		if err != nil {
			return errors.Wrap(err,
				"failed to unmarshal from canonical representation ethereum typed transaction",
			)
		}
		tx.inner = inner
	}
	tx.innerBinarySize = len(canonicalData)
	return nil
}

// EncodeCanonical returns the canonical binary encoding of a transaction.
// For legacy transactions, it returns the RLP encoding. For EIP-2718 typed
// transactions, it returns the type and payload.
func (tx *EthereumTransaction) EncodeCanonical() ([]byte, error) {
	var (
		canonical []byte
		arena     fastrlp.Arena
	)
	if tx.EthereumTxType() == EthereumLegacyTxType {
		fastrlpTx := tx.inner.marshalToFastRLP(&arena)
		canonical = fastrlpTx.MarshalTo(nil)
	} else {
		canonical = tx.encodeTypedCanonical(&arena)
	}
	return canonical, nil
}

// decodeTypedCanonical decodes a typed transaction from the canonical format.
func (tx *EthereumTransaction) decodeTypedCanonical(canonicalData []byte) (EthereumTxData, error) {
	if len(canonicalData) == 0 {
		return nil, errors.New("empty typed transaction bytes")
	}
	switch txType, rlpData := canonicalData[0], canonicalData[1:]; EthereumTxType(txType) {
	case EthereumAccessListTxType:
		var inner EthereumAccessListTx
		if err := inner.DecodeRLP(rlpData); err != nil {
			return nil, errors.Wrapf(err,
				"failed to unmarshal ethereum tx from RLP, ethereumTxType %q",
				EthereumAccessListTxType.String(),
			)
		}
		return &inner, nil
	case EthereumDynamicFeeTxType:
		var inner EthereumDynamicFeeTx
		if err := inner.DecodeRLP(rlpData); err != nil {
			return nil, errors.Wrapf(err,
				"failed to unmarshal ethereum tx from RLP, ethereumTxType %q",
				EthereumDynamicFeeTxType.String(),
			)
		}
		return &inner, nil
	default:
		return nil, ErrTxTypeNotSupported
	}
}

// encodeTypedCanonical returns the canonical encoding of a typed transaction.
func (tx *EthereumTransaction) encodeTypedCanonical(arena *fastrlp.Arena) []byte {
	typedTxVal := tx.inner.marshalToFastRLP(arena)
	canonicalMarshaled := make([]byte, 0, 1+typedTxVal.Len())
	canonicalMarshaled = append(canonicalMarshaled, byte(tx.EthereumTxType()))
	canonicalMarshaled = typedTxVal.MarshalTo(canonicalMarshaled)
	return canonicalMarshaled
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28 && v != 1 && v != 0
	}
	// anything not 27 or 28 is considered protected
	return true
}

// Protected says whether the transaction is replay-protected.
func (tx *EthereumTransaction) Protected() bool {
	switch tx := tx.inner.(type) {
	case *EthereumLegacyTx:
		return tx.V != nil && isProtectedV(tx.V)
	default:
		return true
	}
}