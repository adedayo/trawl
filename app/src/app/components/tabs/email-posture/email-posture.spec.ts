import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EmailPosture } from './email-posture';

describe('EmailPosture', () => {
  let component: EmailPosture;
  let fixture: ComponentFixture<EmailPosture>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [EmailPosture],
    }).compileComponents();

    fixture = TestBed.createComponent(EmailPosture);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
