// Regenerates the store screenshots from the real screens: Play phone
// (1080x1920) and, per App Store locale, 6.9" iPhone (1320x2868) and 13" iPad
// (2064x2752). The scenes and captions are in store_shots.dart.
//   flutter test test_store/store_screenshots_test.dart
import 'package:flutter_test/flutter_test.dart';

import 'store_shots.dart';
import 'support/store_harness.dart';

/// A canvas and every file it is written to for a shot.
typedef Output = ({
  String name,
  StoreTarget target,
  List<String> Function(Shot) paths,
});

List<String> _iosPaths(Shot shot, String device) => [
  for (final locale in iosLocales)
    '${iosScreenshots.path}/$locale/${shot.file}_$device.png',
];

final outputs = <Output>[
  (
    name: 'play phone',
    target: phoneTarget,
    paths: (shot) => [
      '${listingImages.path}/phoneScreenshots/${shot.file}.png',
    ],
  ),
  (
    name: 'iphone 6.9',
    target: iphoneTarget,
    paths: (shot) => _iosPaths(shot, 'iphone'),
  ),
  (
    name: 'ipad 13',
    target: ipadTarget,
    paths: (shot) => _iosPaths(shot, 'ipad'),
  ),
];

void main() {
  setUpAll(() async {
    TestWidgetsFlutterBinding.ensureInitialized();
    await loadBundledFonts();
  });

  for (final output in outputs) {
    for (final shot in shots) {
      testWidgets('${output.name} ${shot.file}', (tester) async {
        await pumpStoreShot(tester, shot: shot, target: output.target);
        for (final path in output.paths(shot)) {
          await savePng(tester, path);
        }
      });
    }
  }
}
