// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

// $BLOBZ tokens can be mined by any transaction that:
// - is sent from an EOA (not a contract)
// - calls the mint() function of this contract
// - contains *exactly one* blob (containing whatever you want!)
//
// The initial amount of tokens emitted per mint() call is 1M, but this amount decreases gradually
// over time. After 30 days, the mint() amount will have decreased to 1M/2, or 500K tokens.
// After 60 days, the amount will reach 1M/3, or 333K tokens. After 90 days, 1M/4, and so on.
// Emissions are reduced when demand for blobs is high by dividing by the blobbasefee in Gwei.
//
// The growth in supply of $BLOBZ is bounded by the logarithm in time elapsed since inception,
// ensuring scarcity.

import {IOptimismPortal2 as IOptimismPortal} from "@optimism/interfaces/L1/IOptimismPortal2.sol";
import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

interface IBlobzToken {
    function mintTo(address to, uint256 amount) external;
}

contract BlobzMinter is Initializable {
    function initialize(address _tokenAddress, address payable _optimismPortalAddress) public initializer {
        TOKEN_ADDRESS = _tokenAddress;
        OPTIMISM_PORTAL_ADDRESS = _optimismPortalAddress;
    }

    // the address of the $BLOBZ token contract on the Base chain
    address internal TOKEN_ADDRESS;
    // the address of the Base OptimismPortal contract on Ethereum
    address payable internal OPTIMISM_PORTAL_ADDRESS;
    // the address to send any ETH sent to this contract
    address payable internal DONATION_ADDRESS = payable(0xb26169eE03Df574eCBA5ca6550462359B587537f);

    // block.timestamp must be greater than START_TIME before minting is allowed
    uint256 internal immutable START_TIME = 1735754400; // 2025-01-01 10:00:00 AM PST
    uint256 internal immutable HALVING_DURATION = 2592000; // 30 days in seconds
    uint256 internal immutable INITIAL_EMISSION_AMOUNT = 1_000_000 * 10 ** 18;

    function _amount() internal view returns (uint256) {
        unchecked {
            uint256 elapsed = block.timestamp - START_TIME;
            uint256 blobBaseFeeDivisor = block.blobbasefee / 1 gwei;
            if (blobBaseFeeDivisor == 0) {
                blobBaseFeeDivisor = 1;
            }
            uint256 mintAmount =
                (INITIAL_EMISSION_AMOUNT * HALVING_DURATION) / (HALVING_DURATION + elapsed) / blobBaseFeeDivisor;
            if (mintAmount == 0) {
                // allow for infinite tail emission after we run out of precision.
                mintAmount = 1;
            }
            return mintAmount;
        }
    }

    // returns the current amount of tokens that would be minted with a successfull mint()
    function amount() public view returns (uint256) {
        if (block.timestamp <= START_TIME) {
            return 0;
        }
        return _amount();
    }

    event Mint(address indexed _to, bytes32 indexed _blobhash, uint256 _value);

    // mints $BLOBZ tokens to the caller's address on Base L2 if the transaction meets the minting criteria
    function mint() public payable {
        mintTo(msg.sender);
    }

    // mints $BLOBZ tokens to the provided address on the Base L2 if the transaction meets the minting criteria
    function mintTo(address _to) public payable {
        require(tx.origin == msg.sender, "not an EOA, no tokens for you!");
        require(blobhash(0) != bytes32(0), "no blob, no tokens for you!");
        require(blobhash(1) == bytes32(0), "too many blobs, no tokens for you!");

        // revert if we haven't reached the start time yet
        require(block.timestamp > START_TIME, "minting not open yet, no tokens for you!");

        if (msg.value > 0) {
            DONATION_ADDRESS.transfer(msg.value); // don't strand ETH sent to this contract
        }

        uint256 mintAmount = _amount();
        _mint(_to, mintAmount);
        emit Mint(_to, blobhash(0), mintAmount);
    }

    function _mint(address to, uint256 amt) internal {
        IOptimismPortal portal = IOptimismPortal(OPTIMISM_PORTAL_ADDRESS);
        portal.depositTransaction({
            _to: TOKEN_ADDRESS,
            _value: 0,
            _gasLimit: 100000,
            _isCreation: false,
            _data: abi.encodeCall(IBlobzToken.mintTo, (to, amt))
        });
    }
}
