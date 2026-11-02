// Minimal 24-bit (RGB, no alpha) PNG encoder: Google Play rejects store
// graphics with an alpha channel, and `dart:ui` only encodes RGBA.
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

/// Encodes straight RGBA [rgba] pixels ([width] x [height]) as an RGB PNG.
/// Pixels are assumed opaque; alpha is dropped.
Uint8List encodeRgbPng(Uint8List rgba, int width, int height) {
  // Each scanline: filter byte 0 followed by RGB triples.
  final raw = Uint8List((width * 3 + 1) * height);
  var o = 0;
  for (var y = 0; y < height; y++) {
    raw[o++] = 0;
    for (var x = 0; x < width; x++) {
      final i = (y * width + x) * 4;
      raw[o++] = rgba[i];
      raw[o++] = rgba[i + 1];
      raw[o++] = rgba[i + 2];
    }
  }
  final header = ByteData(13)
    ..setUint32(0, width)
    ..setUint32(4, height)
    ..setUint8(8, 8) // bit depth
    ..setUint8(9, 2); // colour type: truecolour (RGB)
  final out = BytesBuilder()
    ..add(const [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])
    ..add(_chunk('IHDR', header.buffer.asUint8List()))
    ..add(_chunk('IDAT', Uint8List.fromList(ZLibCodec().encode(raw))))
    ..add(_chunk('IEND', Uint8List(0)));
  return out.toBytes();
}

Uint8List _chunk(String type, Uint8List data) {
  final typed = Uint8List.fromList([...ascii.encode(type), ...data]);
  final out = ByteData(12 + data.length);
  out.setUint32(0, data.length);
  final bytes = out.buffer.asUint8List()..setRange(4, 8 + data.length, typed);
  out.setUint32(8 + data.length, _crc32(typed));
  return bytes;
}

final _crcTable = List<int>.generate(256, (n) {
  var c = n;
  for (var k = 0; k < 8; k++) {
    c = (c & 1) != 0 ? 0xEDB88320 ^ (c >> 1) : c >> 1;
  }
  return c;
});

int _crc32(List<int> bytes) {
  var c = 0xFFFFFFFF;
  for (final b in bytes) {
    c = _crcTable[(c ^ b) & 0xFF] ^ (c >> 8);
  }
  return c ^ 0xFFFFFFFF;
}

/// Writes [rgba] as an RGB PNG to [path] (creating parent directories).
void writeRgbPng(String path, Uint8List rgba, int width, int height) {
  File(path)
    ..createSync(recursive: true)
    ..writeAsBytesSync(encodeRgbPng(rgba, width, height));
}
