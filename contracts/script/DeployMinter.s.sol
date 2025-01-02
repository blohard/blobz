// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.27;

import {console} from "forge-std/console.sol";
import {Script} from "forge-std/Script.sol";
import {BlobzMinter} from "../src/BlobzMinter.sol";

// Deply this contract on mainnet after setting the following environment variables:
// - TOKEN: the address of the $BLOBZ token contract on Base
// - PORTAL: the address of the Base OptimismPortal contract on Ethereum
// - PRIVATE_KEY: the private key of the deployer

contract DeployMinter is Script {
    // address of the $BLOBZ token contract on Base
    address immutable TOKEN_ADDRESS = vm.envAddress("TOKEN");
    // address of Base's OptimismPortal contract on Ethereum
    address payable immutable OPTIMISM_PORTAL_ADDRESS = payable(vm.envAddress("PORTAL"));

    function run() public {
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        vm.startBroadcast(deployerPrivateKey);

        BlobzMinter minter = new BlobzMinter{salt: "LOT O SALT"}();
        minter.initialize(TOKEN_ADDRESS, OPTIMISM_PORTAL_ADDRESS);
        console.log("Minter deployed at", address(minter));

        vm.stopBroadcast();
    }
}
