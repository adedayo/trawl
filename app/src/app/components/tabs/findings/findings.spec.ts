import { ComponentFixture, TestBed } from '@angular/core/testing';

import { FindingsComponent } from './findings';

describe('Findings', () => {
  let component: FindingsComponent;
  let fixture: ComponentFixture<FindingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FindingsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(FindingsComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
