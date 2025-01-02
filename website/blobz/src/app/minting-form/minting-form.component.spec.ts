import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MintingFormComponent } from './minting-form.component';

describe('MintingFormComponent', () => {
  let component: MintingFormComponent;
  let fixture: ComponentFixture<MintingFormComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MintingFormComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MintingFormComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
