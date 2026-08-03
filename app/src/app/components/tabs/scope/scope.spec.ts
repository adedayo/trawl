import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Scope } from './scope';

describe('Scope', () => {
  let component: Scope;
  let fixture: ComponentFixture<Scope>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [Scope],
    }).compileComponents();

    fixture = TestBed.createComponent(Scope);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
