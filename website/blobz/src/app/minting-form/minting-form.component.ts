import { Component, RESPONSE_INIT } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';
import { NgIf } from '@angular/common';

interface MintResponse {
  txid: string;
  address: string;
}

@Component({
  selector: 'app-minting-form',
  standalone: true,
  imports: [FormsModule, NgIf],
  templateUrl: './minting-form.component.html',
  styleUrls: ['./minting-form.component.css']
})
export class MintingFormComponent {
  mintToAddress: string = '';
  privateKey: string = '';
  blobMessage: string = '';
  errorMessage: string = '';
  showError: boolean = false;
  showTxID: boolean = false;
  isLoading: boolean = false;
  txid: string = '';
  address: string = '';
  showSuccess: boolean = false;
  testnetPrefix: string = '';

  constructor(private http: HttpClient) { }

  onSubmit() {
    this.isLoading = true;
    const payload = {
      address: this.mintToAddress,
      pkey: this.privateKey,
      blob: this.blobMessage
    };

    this.http.post<MintResponse>('/json/mint', payload)
      .subscribe({
        next: (response) => {
          console.log('Mint successful:', response);
          this.txid = response.txid;
          this.address = response.address;
          this.showSuccess = true;
          this.isLoading = false;
        },
        error: (error) => {
          this.isLoading = false;
          this.showTxID = false;
          console.error('Mint failed:', error);
          if (!error.error) {
            this.errorMessage = `Oops, something went wrong! (Status: ${error.status} ${error.statusText})`;
            this.showError = true;
          } else {
            switch (error.error.code) {
              case 102:
                this.errorMessage = `The mint-to address is invalid.`
                break;
              case 103:
                this.errorMessage = `You must specify a private key.`
                break;
              case 104:
                this.errorMessage = `The private key is invalid.`
                break;
              case 106:
                this.errorMessage = `The account corresponding to your private key has 0 balance. Please make sure it has enough ETH to cover the gas fee.`
                break;
              case 110:
                this.errorMessage = `Failed to estimate gas. Does the account have enough ETH to cover the gas fee?`
                break;
              case 117:
                this.errorMessage = `Timeout waiting for receipt. It's possible the transaction will still go through.`
                this.txid = error.error.txid;
                this.showTxID = true;
                break;
              default:
                if (error.error.message) {
                  this.errorMessage = `Oops, something went wrong! (Code: ${error.error.code} ${error.error.message})`
                } else {
                  this.errorMessage = `Oops, something went wrong! (Code: ${error.error.code})`;
                }
            }
            this.showError = true;
          }
        }
      });
  }
}
