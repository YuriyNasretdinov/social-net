<?php
$pp = popen('echo Start && ' . __DIR__ . '/notify-' . strtolower(PHP_OS) . ' ' . __DIR__, 'r');
$prog = getenv("GOPATH") . "/bin/social-net";
$cmd = "exec sudo -u web " . $prog;
$ph = proc_open($cmd, array(STDIN, STDOUT, STDERR), $pipes);;
register_shutdown_function(function() { global $pp, $ph; proc_terminate($ph); pclose($pp); });

while (true) {
	$ln = fgets($pp);
	if ($ln === false) exit(0);

	echo "\nBuilding...\n";
	$start = microtime(true);
	system("go install -v && sudo setcap 'cap_net_bind_service=+ep' " . $prog, $retval);
	echo "Done in " . round(microtime(true) - $start, 2) . " sec\n\n";
	if ($retval == 0) {
		echo "Success!\n";
		echo "Terminating\n";
		system("sudo -u web killall social-net");
		echo "Closing\n";
        proc_close($ph);
		echo $cmd . "\n";
		$ph = proc_open($cmd, array(STDIN, STDOUT, STDERR), $pipes);
	} else {
		echo "Build failed\n";
	}
	
	sleep(3);

	stream_set_blocking($pp, false);
	while (fgets($pp) !== false);
	stream_set_blocking($pp, true);
}
