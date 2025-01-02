import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MintingStatsComponent } from './minting-stats.component';

describe('MintingStatsComponent', () => {
  let component: MintingStatsComponent;
  let fixture: ComponentFixture<MintingStatsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MintingStatsComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MintingStatsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
