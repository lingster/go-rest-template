// Regenerates the Play feature graphic (1024x500, 24-bit PNG, no alpha). The
// name, tagline and colours are in store_shots.dart; the mark is the adaptive
// icon foreground (assets/icon/icon-foreground.png).
//   flutter test test_store/feature_graphic_test.dart
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'store_shots.dart';
import 'support/store_harness.dart';

const _size = Size(1024, 500);
const _markFile = 'assets/icon/icon-foreground.png';

/// The app mark cropped to its artwork so it can be sized freely.
class _Mark extends StatelessWidget {
  const _Mark({required this.height});

  final double height;

  // The default mark occupies 398x460 of the 1024x1024 artwork, centred.
  // Change these if you swap in a different icon-foreground.png.
  static const _w = 398 / 1024, _h = 460 / 1024;

  @override
  Widget build(BuildContext context) {
    final full = height / _h;
    return SizedBox(
      width: full * _w,
      height: height,
      child: ClipRect(
        child: OverflowBox(
          maxWidth: full,
          maxHeight: full,
          child: Image.file(File(_markFile), width: full, height: full),
        ),
      ),
    );
  }
}

class _FeatureGraphic extends StatelessWidget {
  const _FeatureGraphic();

  @override
  Widget build(BuildContext context) {
    const font = TextStyle(
      fontFamily: 'Roboto',
      color: Colors.white,
      decoration: TextDecoration.none,
    );
    return Directionality(
      textDirection: TextDirection.ltr,
      child: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [brandDark, brandLight],
          ),
        ),
        padding: const EdgeInsets.symmetric(horizontal: 96),
        child: Row(
          children: [
            const _Mark(height: 280),
            const SizedBox(width: 72),
            Expanded(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    storeAppName,
                    maxLines: 2,
                    style: font.copyWith(
                      fontSize: 84,
                      fontWeight: FontWeight.w700,
                      height: 1.05,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    storeTagline,
                    maxLines: 2,
                    style: font.copyWith(
                      fontSize: 38,
                      fontWeight: FontWeight.w500,
                      color: brandAccent,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

void main() {
  setUpAll(() async {
    TestWidgetsFlutterBinding.ensureInitialized();
    await loadBundledFonts();
  });

  testWidgets('feature graphic', (tester) async {
    useStoreView(tester, size: _size, logicalWidth: _size.width);
    await tester.runAsync(() async {
      await tester.pumpWidget(
        RepaintBoundary(key: shotKey, child: const _FeatureGraphic()),
      );
      // Decode the mark before the frame is captured.
      await precacheImage(
        FileImage(File(_markFile)),
        tester.element(find.byType(_FeatureGraphic)),
      );
    });
    await tester.pumpAndSettle();
    await savePng(tester, '${listingImages.path}/featureGraphic.png');
  });
}
