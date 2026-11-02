// Renders real app screens into store-sized PNGs. Not part of `flutter test`
// (lives outside test/); run with `flutter test test_store`. See the README,
// "Customise your store listing".
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:saas_app/app.dart';
import 'package:saas_app/router.dart';

import '../store_shots.dart';
import 'fake_store_app.dart';
import 'rgb_png.dart';

/// A store screenshot slot: the canvas in pixels, the logical width the app
/// is laid out at (so the pixel ratio follows), and how much to scale the
/// caption band against a phone.
class StoreTarget {
  const StoreTarget(this.size, this.logicalWidth, {this.captionScale = 1});

  final Size size;
  final double logicalWidth;
  final double captionScale;
}

/// Play phone screenshot: 1080x1920 (9:16) from a 412-dp-wide screen.
const phoneTarget = StoreTarget(Size(1080, 1920), 412);

/// App Store 6.9" iPhone: 1320x2868 at 3x (440 dp wide).
const iphoneTarget = StoreTarget(Size(1320, 2868), 440, captionScale: 1.1);

/// App Store 13" iPad: 2064x2752. Laid out 688 dp wide (3x) rather than at the
/// real 1032 dp: the app is built from phone-width forms, and this keeps them
/// filling the screenshot instead of floating in a wide empty canvas.
const ipadTarget = StoreTarget(Size(2064, 2752), 688, captionScale: 1.5);

/// Where Play listing assets live (also holds the text files).
final listingImages = Directory(
  '${Directory.current.path}/android/fastlane/metadata/android/en-US/images',
);

/// Where deliver reads App Store screenshots: `<dir>/<locale>/`.
final iosScreenshots = Directory(
  '${Directory.current.path}/ios/fastlane/screenshots',
);

/// Roboto, the Material default on Android, from the Flutter SDK's own
/// artifacts: tests otherwise draw every glyph as a box.
Future<void> _loadRoboto() async {
  final root = Platform.environment['FLUTTER_ROOT'];
  if (root == null) return;
  final dir = Directory('$root/bin/cache/artifacts/material_fonts');
  final fonts = dir.existsSync()
      ? dir.listSync().whereType<File>().where(
          (f) => RegExp(r'Roboto-[A-Za-z]+\.ttf$').hasMatch(f.path),
        )
      : const <File>[];
  final loader = FontLoader('Roboto');
  for (final f in fonts) {
    loader.addFont(Future.value(ByteData.sublistView(f.readAsBytesSync())));
  }
  await loader.load();
}

/// Loads Roboto and every font of the app's FontManifest (Material Icons, ...)
/// so text and icons render as on a device instead of as test boxes.
Future<void> loadBundledFonts() async {
  await _loadRoboto();
  final manifest =
      jsonDecode(await rootBundle.loadString('FontManifest.json')) as List;
  for (final family in manifest.cast<Map<String, dynamic>>()) {
    final loader = FontLoader(family['family'] as String);
    for (final font in (family['fonts'] as List).cast<Map<String, dynamic>>()) {
      loader.addFont(rootBundle.load(font['asset'] as String));
    }
    await loader.load();
  }
}

/// Sets the test view to [size] pixels, laid out [logicalWidth] dp wide.
void useStoreView(
  WidgetTester tester, {
  Size size = const Size(1080, 1920),
  double logicalWidth = 412,
}) {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = size.width / logicalWidth;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

/// Wrap whatever [savePng] should capture in a `RepaintBoundary(key: shotKey)`.
final shotKey = GlobalKey();

/// Boots the real app with fake providers and opens [shot]'s route.
Future<void> pumpStoreShot(
  WidgetTester tester, {
  required Shot shot,
  StoreTarget target = phoneTarget,
}) async {
  useStoreView(tester, size: target.size, logicalWidth: target.logicalWidth);
  debugDisableShadows = false; // flutter_test draws shadows as hard blocks
  FocusManager.instance.highlightStrategy =
      FocusHighlightStrategy.alwaysTouch; // no keyboard focus rings
  WidgetsApp.debugAllowBannerOverride = false; // no DEBUG ribbon
  addTearDown(() => WidgetsApp.debugAllowBannerOverride = true);
  await tester.pumpWidget(
    RepaintBoundary(
      key: shotKey,
      child: ProviderScope(
        retry: (_, _) => null,
        overrides: storeOverrides(signedIn: shot.signedIn),
        child: _Framed(
          caption: shot.caption,
          scale: target.captionScale,
          child: const App(),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  final container = ProviderScope.containerOf(tester.element(find.byType(App)));
  container.read(routerProvider).go(shot.route);
  await tester.pumpAndSettle();
}

/// On-brand caption band above the app.
class _Framed extends StatelessWidget {
  const _Framed({
    required this.child,
    required this.caption,
    required this.scale,
  });

  final Widget child;
  final String caption;
  final double scale;

  @override
  Widget build(BuildContext context) {
    return Directionality(
      textDirection: TextDirection.ltr,
      child: Column(
        children: [
          Container(
            width: double.infinity,
            height: 72 * scale,
            padding: EdgeInsets.symmetric(horizontal: 24 * scale),
            alignment: Alignment.centerLeft,
            decoration: const BoxDecoration(
              gradient: LinearGradient(colors: [brandDark, brandLight]),
            ),
            child: Text(
              caption,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontFamily: 'Roboto',
                fontWeight: FontWeight.w700,
                fontSize: 22 * scale,
                height: 1.15,
                color: Colors.white,
                decoration: TextDecoration.none,
              ),
            ),
          ),
          Expanded(child: child),
        ],
      ),
    );
  }
}

/// Writes the captured widget tree as an RGB PNG at [path].
Future<void> savePng(WidgetTester tester, String path) async {
  await tester.runAsync(() async {
    final boundary =
        shotKey.currentContext!.findRenderObject()! as RenderRepaintBoundary;
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    final rgba = (await image.toByteData())!;
    writeRgbPng(path, rgba.buffer.asUint8List(), image.width, image.height);
  });
  debugDisableShadows = true; // flutter_test checks this is restored
}
