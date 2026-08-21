# Signal Web mobile packages

- Android: Trusted Web Activity package `com.tripvn.msg`, backed by `https://msg.trip-vn.com/`. Web Push is delivered by Chrome without exposing encrypted message content to the application server.
- iOS: `SignalWeb-iOS.mobileconfig` installs the HTTPS web app on the Home Screen. On iOS 16.4 or newer, open the installed app and enable notifications to receive standards-based Web Push.
- A native `.ipa` requires an Apple Developer Team, distribution certificate, provisioning profile, and APNs credentials. Those credentials must not be committed to Git.

The Android release APK is signed with the deployment keystore. Its SHA-256 signing certificate fingerprint is published in `web/client/.well-known/assetlinks.json`; keep using the same private keystore for every upgrade.
