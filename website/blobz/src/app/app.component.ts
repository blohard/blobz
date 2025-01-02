import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { MintingFormComponent } from './minting-form/minting-form.component';
import { MintingStatsComponent } from './minting-stats/minting-stats.component';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, MintingFormComponent, MintingStatsComponent],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent {
}
