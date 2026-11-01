import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:saas_app/core/network/api_exception.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';

import '../../helpers/fake_adapter.dart';
import '../../helpers/pump_app.dart';

User _user() => User.fromJson(userJson());

AuthSuccess _success() =>
    AuthSuccess(tokens: Tokens.fromJson(tokensJson()), user: _user());

Future<void> _fillAndSubmit(WidgetTester tester) async {
  await tester.enterText(
    find.widgetWithText(TextFormField, 'Email'),
    'a@b.com',
  );
  await tester.enterText(
    find.widgetWithText(TextFormField, 'Password'),
    'password1',
  );
  await tester.tap(find.widgetWithText(FilledButton, 'Sign in'));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() {
    registerFallbackValue('');
  });

  testWidgets('starts on login when there is no session', (tester) async {
    await pumpApp(tester);
    expect(find.text('Sign in'), findsWidgets);
    expect(find.byKey(const Key('passkey_signin')), findsOneWidget);
  });

  testWidgets('validates empty and malformed input', (tester) async {
    final repo = await pumpApp(tester);
    await tester.tap(find.widgetWithText(FilledButton, 'Sign in'));
    await tester.pump();
    expect(find.text('Email is required'), findsOneWidget);
    expect(find.text('Password is required'), findsOneWidget);

    await tester.enterText(
      find.widgetWithText(TextFormField, 'Email'),
      'not-an-email',
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Sign in'));
    await tester.pump();
    expect(find.text('Enter a valid email address'), findsOneWidget);
    verifyNever(() => repo.login(any(), any()));
  });

  testWidgets('shows the API error message', (tester) async {
    final repo = await pumpApp(tester);
    when(() => repo.login(any(), any())).thenThrow(
      const ApiException(
        code: ApiErrorCode.invalidCredentials,
        message: 'Invalid email or password',
        statusCode: 401,
      ),
    );
    await _fillAndSubmit(tester);
    expect(find.text('Invalid email or password'), findsOneWidget);
    expect(find.byKey(const Key('resend_verification')), findsNothing);
  });

  testWidgets('email_not_verified offers resending the verification email', (
    tester,
  ) async {
    final repo = await pumpApp(tester);
    when(() => repo.login(any(), any())).thenThrow(
      const ApiException(
        code: ApiErrorCode.emailNotVerified,
        message: 'Email not verified',
        statusCode: 403,
      ),
    );
    when(() => repo.resendVerification(any())).thenAnswer((_) async => 'sent');
    await _fillAndSubmit(tester);
    expect(find.byKey(const Key('resend_verification')), findsOneWidget);

    await tester.tap(find.byKey(const Key('resend_verification')));
    await tester.pumpAndSettle();
    verify(() => repo.resendVerification('a@b.com')).called(1);
    expect(find.byKey(const Key('info_banner')), findsOneWidget);
  });

  testWidgets('navigates to 2FA on MFA challenge and completes login', (
    tester,
  ) async {
    final repo = await pumpApp(tester);
    when(() => repo.login(any(), any())).thenAnswer(
      (_) async => const MfaChallenge(mfaToken: 'mfa', methods: ['totp']),
    );
    when(() => repo.loginWithMfa('mfa', '123456'))
        .thenAnswer((_) async => _success());
    await _fillAndSubmit(tester);
    expect(find.text('Two-factor authentication'), findsOneWidget);

    // Recovery code toggle swaps the field label.
    await tester.tap(find.byKey(const Key('toggle_recovery')));
    await tester.pump();
    expect(find.text('Recovery code'), findsOneWidget);
    await tester.tap(find.byKey(const Key('toggle_recovery')));
    await tester.pump();

    await tester.enterText(find.byType(TextFormField), '123456');
    await tester.tap(find.widgetWithText(FilledButton, 'Verify'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Hello, Ada'), findsOneWidget);
  });

  testWidgets('successful login lands on home', (tester) async {
    final repo = await pumpApp(tester);
    when(() => repo.login(any(), any())).thenAnswer((_) async => _success());
    await _fillAndSubmit(tester);
    expect(find.textContaining('Hello, Ada'), findsOneWidget);
  });

  testWidgets('password visibility toggles', (tester) async {
    await pumpApp(tester);
    EditableText field() => tester.widget<EditableText>(
      find.descendant(
        of: find.widgetWithText(TextFormField, 'Password'),
        matching: find.byType(EditableText),
      ),
    );
    expect(field().obscureText, isTrue);
    await tester.tap(find.byTooltip('Show password'));
    await tester.pump();
    expect(field().obscureText, isFalse);
  });
}
