import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:saas_app/core/network/api_exception.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';

import '../../helpers/fake_adapter.dart';
import '../../helpers/pump_app.dart';

Future<void> _openRegister(WidgetTester tester) async {
  await tester.tap(find.text("Don't have an account? Create one"));
  await tester.pumpAndSettle();
}

Finder _field(String label) => find.widgetWithText(TextFormField, label);

void main() {
  setUpAll(() {
    registerFallbackValue('');
  });

  testWidgets('validates name, email, password policy and confirmation', (
    tester,
  ) async {
    await pumpApp(tester);
    await _openRegister(tester);

    await tester.tap(find.widgetWithText(FilledButton, 'Create account'));
    await tester.pump();
    expect(find.text('Name is required'), findsOneWidget);
    expect(find.text('Email is required'), findsOneWidget);
    expect(find.text('Password is required'), findsOneWidget);

    await tester.enterText(_field('Name'), 'Ada');
    await tester.enterText(_field('Email'), 'a@b.com');
    await tester.enterText(_field('Password'), 'short');
    await tester.enterText(_field('Confirm password'), 'different');
    await tester.tap(find.widgetWithText(FilledButton, 'Create account'));
    await tester.pump();
    expect(find.text('Use at least 8 characters'), findsOneWidget);
    expect(find.text('Passwords do not match'), findsOneWidget);
  });

  testWidgets('submits and shows the success message', (tester) async {
    final repo = await pumpApp(tester);
    when(
      () => repo.register(
        email: any(named: 'email'),
        password: any(named: 'password'),
        name: any(named: 'name'),
      ),
    ).thenAnswer(
      (_) async => RegisterResult(
        user: User.fromJson(userJson()),
        message: 'Please verify your email',
      ),
    );
    await _openRegister(tester);
    await tester.enterText(_field('Name'), 'Ada');
    await tester.enterText(_field('Email'), 'a@b.com');
    await tester.enterText(_field('Password'), 'password1');
    await tester.enterText(_field('Confirm password'), 'password1');
    await tester.tap(find.widgetWithText(FilledButton, 'Create account'));
    await tester.pumpAndSettle();

    verify(
      () => repo.register(email: 'a@b.com', password: 'password1', name: 'Ada'),
    ).called(1);
    expect(find.text('Please verify your email'), findsOneWidget);
  });

  testWidgets('shows field details for validation_error', (tester) async {
    final repo = await pumpApp(tester);
    when(
      () => repo.register(
        email: any(named: 'email'),
        password: any(named: 'password'),
        name: any(named: 'name'),
      ),
    ).thenThrow(
      const ApiException(
        code: ApiErrorCode.conflict,
        message: 'Email already registered',
        statusCode: 409,
      ),
    );
    await _openRegister(tester);
    await tester.enterText(_field('Name'), 'Ada');
    await tester.enterText(_field('Email'), 'a@b.com');
    await tester.enterText(_field('Password'), 'password1');
    await tester.enterText(_field('Confirm password'), 'password1');
    await tester.tap(find.widgetWithText(FilledButton, 'Create account'));
    await tester.pumpAndSettle();
    expect(find.text('Email already registered'), findsOneWidget);
  });
}
