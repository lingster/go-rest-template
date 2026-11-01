import '../../../core/network/api_client.dart';
import '../../auth/data/auth_models.dart';

class ProfileRepository {
  ProfileRepository(this._api);

  final ApiClient _api;

  Future<User> updateName(String name) => _api.patch(
    '/me',
    body: {'name': name},
    decode: (j) => User.fromJson(asMap(j)),
  );

  Future<String> changePassword({
    required String currentPassword,
    required String newPassword,
  }) => _api.post(
    '/me/change-password',
    body: {'current_password': currentPassword, 'new_password': newPassword},
    decode: decodeMessage,
  );
}
