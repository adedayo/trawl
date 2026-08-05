import { ComponentFixture, TestBed } from '@angular/core/testing';

import { AssetsComponent } from './assets';

describe('Assets', () => {
  let component: AssetsComponent;
  let fixture: ComponentFixture<AssetsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AssetsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(AssetsComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
