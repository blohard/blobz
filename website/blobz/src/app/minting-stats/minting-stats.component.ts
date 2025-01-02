import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { HttpClient } from '@angular/common/http';

@Component({
  selector: 'app-minting-stats',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './minting-stats.component.html',
  styleUrls: ['./minting-stats.component.css']
})
export class MintingStatsComponent implements OnInit, OnDestroy {
  mintAmount: string = '0.00';
  estimatedGasFee: string = '0.0';
  blobCongestion: number = 0;
  private updateInterval: any;

  constructor(private http: HttpClient) { }

  ngOnInit() {
    // Update stats every 12 seconds
    this.updateInterval = setInterval(() => {
      this.updateStats();
    }, 12000);

    // Initial update
    this.updateStats();
  }

  ngOnDestroy() {
    if (this.updateInterval) {
      clearInterval(this.updateInterval);
    }
  }

  private updateStats() {
    this.http.get<any>('/json/stats')
      .subscribe({
        next: (data) => {
          this.mintAmount = Number(data.mint_amount || 0).toFixed(2);
          this.estimatedGasFee = Number(data.estimated_fee || 0).toPrecision(3);
          this.blobCongestion = data.congestion_factor || 0;
        },
        error: (error) => {
          console.error('Error fetching stats:', error);
          this.mintAmount = '?';
          this.estimatedGasFee = '?';
          this.blobCongestion = -1;
        }
      });
  }
}
