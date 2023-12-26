package itests

import (
	"testing"

	"github.com/stretchr/testify/suite"
	f "github.com/wavesplatform/gowaves/itests/fixtures"
	"github.com/wavesplatform/gowaves/itests/testdata"
	utl "github.com/wavesplatform/gowaves/itests/utilities"
	"github.com/wavesplatform/gowaves/itests/utilities/issue_utilities"
	"github.com/wavesplatform/gowaves/itests/utilities/sponsor_utilities"
	"github.com/wavesplatform/gowaves/itests/utilities/transfer_utilities"
	"github.com/wavesplatform/gowaves/pkg/crypto"
)

type SponsorshipTxSuite struct {
	f.BaseSuite
}

func (suite *SponsorshipTxSuite) TestSponsorshipTxPositive() {
	versions := sponsor_utilities.GetVersions(&suite.BaseSuite)
	waitForTx := true
	for _, v := range versions {
		reissuable := testdata.GetCommonIssueData(&suite.BaseSuite).Reissuable
		itx := issue_utilities.IssueSendWithTestData(&suite.BaseSuite, reissuable, v, waitForTx)
		tdmatrix := testdata.GetSponsorshipPositiveDataMatrix(&suite.BaseSuite, itx.TxID)
		for name, td := range tdmatrix {
			caseName := utl.GetTestcaseNameWithVersion(name, v)
			suite.Run(caseName, func() {
				tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset := sponsor_utilities.SendSponsorshipTxAndGetBalances(
					&suite.BaseSuite, td, v, waitForTx)
				errMsg := caseName + "Sponsorship tx: " + tx.TxID.String()

				utl.TxInfoCheck(suite.T(), tx.WtErr.ErrWtGo, tx.WtErr.ErrWtScala, errMsg)
				utl.WavesDiffBalanceCheck(suite.T(), td.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
					actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
				utl.AssetDiffBalanceCheck(suite.T(), td.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
					actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)
			})
		}
	}
}

func (suite *SponsorshipTxSuite) TestSponsorshipTxMaxValues() {
	versions := sponsor_utilities.GetVersions(&suite.BaseSuite)
	waitForTx := true
	for _, v := range versions {
		n := transfer_utilities.GetNewAccountWithFunds(&suite.BaseSuite, v, utl.TestChainID,
			utl.DefaultAccountForLoanFunds, 10000000000)
		itxID := issue_utilities.IssueAssetAmount(&suite.BaseSuite, v, utl.TestChainID,
			utl.DefaultSenderNotMiner, utl.MaxAmount)
		transfer_utilities.TransferAssetAmount(&suite.BaseSuite, v, utl.TestChainID, itxID, utl.DefaultSenderNotMiner, n)
		tdmatrix := testdata.GetSponsorshipMaxValuesPositive(&suite.BaseSuite, itxID, n)
		for name, td := range tdmatrix {
			caseName := utl.GetTestcaseNameWithVersion(name, v)
			suite.Run(caseName, func() {
				tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset := sponsor_utilities.SendSponsorshipTxAndGetBalances(
					&suite.BaseSuite, td, v, waitForTx)
				errMsg := caseName + "Sponsorship tx: " + tx.TxID.String()

				utl.TxInfoCheck(suite.T(), tx.WtErr.ErrWtGo, tx.WtErr.ErrWtScala, errMsg)
				utl.WavesDiffBalanceCheck(suite.T(), td.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
					actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
				utl.AssetDiffBalanceCheck(suite.T(), td.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
					actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)
			})
		}
	}
}

func (suite *SponsorshipTxSuite) TestSponsorshipDisabledTx() {
	versions := sponsor_utilities.GetVersions(&suite.BaseSuite)
	name := "Sponsorship Enabled/Disabled"
	waitForTx := true
	for _, v := range versions {
		reissuable := testdata.GetCommonIssueData(&suite.BaseSuite).Reissuable
		itx := issue_utilities.IssueSendWithTestData(&suite.BaseSuite, reissuable, v, waitForTx)
		sponsorship := testdata.GetSponsorshipEnabledDisabledData(&suite.BaseSuite, itx.TxID)
		caseName := utl.GetTestcaseNameWithVersion(name, v)
		suite.Run(caseName, func() {
			//switch on sponsorship
			tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset := sponsor_utilities.SendSponsorshipTxAndGetBalances(
				&suite.BaseSuite, sponsorship.Enabled, v, waitForTx)
			errMsg := caseName + "Sponsorship tx: " + tx.TxID.String()

			utl.TxInfoCheck(suite.T(), tx.WtErr.ErrWtGo, tx.WtErr.ErrWtScala, errMsg)
			utl.WavesDiffBalanceCheck(suite.T(), sponsorship.Enabled.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
				actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
			utl.AssetDiffBalanceCheck(suite.T(), sponsorship.Enabled.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
				actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)

			//switch off sponsorship
			tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset = sponsor_utilities.SendSponsorshipTxAndGetBalances(
				&suite.BaseSuite, sponsorship.Disabled, v, waitForTx)
			errMsg = caseName + "Sponsorship Disabled tx: " + tx.TxID.String()

			utl.TxInfoCheck(suite.T(), tx.WtErr.ErrWtGo, tx.WtErr.ErrWtScala, errMsg)
			utl.WavesDiffBalanceCheck(suite.T(), sponsorship.Disabled.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
				actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
			utl.AssetDiffBalanceCheck(suite.T(), sponsorship.Disabled.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
				actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)
		})
	}
}

func (suite *SponsorshipTxSuite) TestSponsorshipTxNegative() {
	versions := sponsor_utilities.GetVersions(&suite.BaseSuite)
	waitForTx := true
	txIds := make(map[string]*crypto.Digest)

	for _, v := range versions {
		reissuable := testdata.GetCommonIssueData(&suite.BaseSuite).Reissuable
		itx := issue_utilities.IssueSendWithTestData(&suite.BaseSuite, reissuable, v, waitForTx)
		tdmatrix := testdata.GetSponsorshipNegativeDataMatrix(&suite.BaseSuite, itx.TxID)

		for name, td := range tdmatrix {
			caseName := utl.GetTestcaseNameWithVersion(name, v)
			suite.Run(caseName, func() {
				tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset := sponsor_utilities.SendSponsorshipTxAndGetBalances(
					&suite.BaseSuite, td, v, !waitForTx)
				errMsg := caseName + "Sponsorship tx: " + tx.TxID.String()

				txIds[name] = &tx.TxID

				utl.ErrorMessageCheck(suite.T(), td.Expected.ErrGoMsg, td.Expected.ErrScalaMsg, tx.WtErr.ErrWtGo,
					tx.WtErr.ErrWtScala, errMsg)
				utl.WavesDiffBalanceCheck(suite.T(), td.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
					actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
				utl.AssetDiffBalanceCheck(suite.T(), td.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
					actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)
			})
		}
	}
	actualTxIds := utl.GetTxIdsInBlockchain(&suite.BaseSuite, txIds)
	suite.Lenf(actualTxIds, 0, "IDs: %#v", actualTxIds)
}

func (suite *SponsorshipTxSuite) Test_SponsorshipForSmartAssetNegative() {
	versions := sponsor_utilities.GetVersions(&suite.BaseSuite)
	waitForTx := true
	txIds := make(map[string]*crypto.Digest)
	for _, v := range versions {
		smart := testdata.GetCommonIssueData(&suite.BaseSuite).Smart
		itx := issue_utilities.IssueSendWithTestData(&suite.BaseSuite, smart, v, waitForTx)
		td := testdata.GetSponsorshipForSmartAssetData(&suite.BaseSuite, itx.TxID).Enabled
		name := "Check sponsorship for smart asset"
		caseName := utl.GetTestcaseNameWithVersion(name, v)

		suite.Run(caseName, func() {
			tx, actualDiffBalanceInWaves, actualDiffBalanceInAsset := sponsor_utilities.SendSponsorshipTxAndGetBalances(
				&suite.BaseSuite, td, v, !waitForTx)
			errMsg := caseName + "Sponsorship tx: " + tx.TxID.String()
			txIds[name] = &tx.TxID

			utl.ErrorMessageCheck(suite.T(), td.Expected.ErrGoMsg, td.Expected.ErrScalaMsg, tx.WtErr.ErrWtGo,
				tx.WtErr.ErrWtScala, errMsg)
			utl.WavesDiffBalanceCheck(suite.T(), td.Expected.WavesDiffBalance, actualDiffBalanceInWaves.BalanceInWavesGo,
				actualDiffBalanceInWaves.BalanceInWavesScala, errMsg)
			utl.AssetDiffBalanceCheck(suite.T(), td.Expected.AssetDiffBalance, actualDiffBalanceInAsset.BalanceInAssetGo,
				actualDiffBalanceInAsset.BalanceInAssetScala, errMsg)
		})
	}
	actualTxIds := utl.GetTxIdsInBlockchain(&suite.BaseSuite, txIds)
	suite.Lenf(actualTxIds, 0, "IDs: %#v", actualTxIds)
}

func TestSponsorshipTxSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SponsorshipTxSuite))
}