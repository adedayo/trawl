import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ScopeComponent } from './scope';

describe('Scope', () => {
  let component: ScopeComponent;
  let fixture: ComponentFixture<ScopeComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ScopeComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ScopeComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
