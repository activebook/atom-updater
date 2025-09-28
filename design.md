
# Context:
1. Download the latest version of the app from the server.
2. Compare the current version with the downloaded version.
3. If the downloaded version is newer, extract the new version and replace the current version.
4. If the downloaded version is older, do nothing.
5. If the downloaded version is the same, do nothing.
6. If the downloaded version is not available, do nothing.
7. If the downloaded version is corrupted, do nothing.

# Constriction & limitation: 
i'm only on github, there isn't self hosted sever.
where to check the version? 
where to download?
when to check the version?
so i think the best practice is:
1. Check the version of the current version after started.
2, Using git url to check remote version
3, Get asset and download it
4, Check the downloaded sum comparing with sha checksum
5, Using downloaded version to replact itself.

but it implys more questions:
1, manual installation is not a possible selection! 
2, write a shell to replace itself? how window runs that bash shell? 3, Apply update before fully initializing the app??? the app is ready, how could that be possible?

i think we need a helper app bundled into the electron extraResources and electron app need to copy it to a safe place, for updating use, and run it to replace itself.
here is what it would do:
Wait for app exit (poll PID or lock file).
Rename old binary/bundle → app.old.
Copy staged new version into app/.
Verify checksum/signature again after copy.
Relaunch app from the canonical name (app.exe / MyApp.app).
Delete old version once the new one has passed a health check.

Key Advantages of This Approach
Clean separation: Updater is independent of main app
Atomic operations: All-or-nothing replacement
Rollback capability: Can restore previous version if update fails
Health verification: Ensures new version starts properly
Cross-platform: Works on macOS, Windows, and Linux
No file locking issues: Updater runs when main app has exited

the helper app requires parameters:
1, pid (wait to exit to do the atomic replacement)
2, path to the current version
3, path to the new version

after updating, it will open the new version of app and exit itself.

how electron to check the updater app version:
package.json store the updater app version.
spawn it, and wait for "updater --version" output.
if the version is the same, then it's a no-op.
if the version is older, copy the new version, which in the extraResources over the old one.

so here we have a design for the helper app:

# design for updating atom-updater app itself:

## atom-updater:
an go app that can update apps themself.

## target:
running app cannot update itself, so it needs a helper app.
that what this app does.

## functionalities:

- version check:
atom-updater -v|--version
return the version of atom-updater
if target app find the atom-updater is old, it will copy its packaged new version to overwrite the old one.

- update the app itself:
atom-updater <pid> <current_version_path> <new_version_path>
1, pid (wait to exit to do the atomic replacement)
2, path to the current version
3, path to the new version

atom-updater will wait the pid to exit, and then copy the new version to overwrite the old one.
if would first change the old version off the app to app.tmp, and then copy the new version to app.new, and then change the app.new to app. and then remove the app.tmp.
the suffix tmp and new are all calculated by algorithm, to keep unique.
after updating, it will open the new version of app and exit atom-updater itself.
if any those actions failed, it will change the app.tmp to app. roll back the old version.

# keypoints:
1. atom-updater is a go app, it can update itself by host app to check the version, and copy the new version to overwrite the old one.
2, atom-updater must wait the host app to exit, because running one cannot be updated.
3, atom-updater must change the old version of the app's name, because it cannot let user click it to open, because when update, it cannot be opened again.
4, atom-updater must not fail, always roll back to the old version when failed.

# clarification:

- How the updater determines what version of the target app to download?
the target app would privide a parameter to tell the updater what new version path used to copy from.
atom-updater doesn't need to know whether that's new or not, it just doing the copy&replace job.

- Whether the updater version and target app version are independent?
Yes! the major aim of spawn atom-updater is to update the target itself, so it's independent.
but because in target app, there would be atom-updater version number, for example, in electron app's package.json, so the target app could check the atom-updater version(atom-updater -v) and compare it with the target app's atom-updater version, and decide whether to update or not.

- How version metadata is structured in releases?
The version metadata is stored in the release assets of remote github.
not in self-hosted server.

- How will the initial version of atom-updater be distributed?
because the target app packaged with atom-updater, so it's easy to distribute. first time check the would-be installed/copied atom-updater path, if there isn't then copy the packaged one to that path.

- What constitutes a successful health check?
the new version of the target app can be opened.
maybe wait for a new pid? then atom-updater can quit itself.

- Cross-platform support?
yes!

# implementation:
What this project does is to create a go app "atom-updater"!