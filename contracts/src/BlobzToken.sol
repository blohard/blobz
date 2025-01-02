// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@solmate/tokens/ERC20.sol";
import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";

contract BlobzToken is ERC20, Initializable {
    constructor() ERC20("Blobz", "BLOBZ", 18) {}

    // initialize with the *aliased* address of the minter contract on Ethereum
    function initialize(address _minterAddress) public initializer {
        MINTER_ADDRESS = _minterAddress;
    }

    address internal MINTER_ADDRESS; // aliased address of the minter contract on Ethereum

    function mintTo(address to, uint256 amount) public {
        require(msg.sender == MINTER_ADDRESS, "only the minter contract can mint, no tokens for you!");
        _mint(to, amount);
    }
}
