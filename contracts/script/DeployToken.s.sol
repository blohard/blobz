// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.27;

import {console} from "forge-std/console.sol";
import {Script} from "forge-std/Script.sol";
import {BlobzToken} from "../src/BlobzToken.sol";

// Deploy this contract on Base after setting the following environment variables:
// - MINTER: the address of the BlobzMinter contract on Ethereum
// - PRIVATE_KEY: the private key of the deployer

contract DeployToken is Script {
    // address of the BlobzMinter contract on Ethereum
    address internal MINTER_ADDRESS = vm.envAddress("MINTER");

    function run() public {
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        vm.startBroadcast(deployerPrivateKey);

        BlobzToken blobz = new BlobzToken{salt: "LOT O SALT"}();
        console.log("Blobz deployed at", address(blobz));

        uint160 minter160 = uint160(MINTER_ADDRESS);
        // account for aliasing
        unchecked {
            MINTER_ADDRESS = address(minter160 + uint160(0x1111000000000000000000000000000000001111));
        }
        console.log("Aliased minter address:", MINTER_ADDRESS);
        blobz.initialize(MINTER_ADDRESS);

        vm.stopBroadcast();
    }
}
