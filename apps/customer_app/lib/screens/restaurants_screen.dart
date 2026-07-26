import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api.dart';
import 'login_screen.dart';
import 'menu_screen.dart';

class RestaurantsScreen extends StatefulWidget {
  const RestaurantsScreen({super.key});

  @override
  State<RestaurantsScreen> createState() => _RestaurantsScreenState();
}

class _RestaurantsScreenState extends State<RestaurantsScreen> {
  late Future<List<dynamic>> _future;

  @override
  void initState() {
    super.initState();
    _future = api.restaurants();
  }

  Future<void> _logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('token');
    api.token = null;
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Restoranlar'),
        actions: [
          IconButton(onPressed: _logout, icon: const Icon(Icons.logout)),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async => setState(() => _future = api.restaurants()),
        child: FutureBuilder<List<dynamic>>(
          future: _future,
          builder: (context, snap) {
            if (snap.connectionState != ConnectionState.done) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snap.hasError) {
              return Center(child: Text('Xato: ${snap.error}'));
            }
            final list = snap.data ?? [];
            if (list.isEmpty) {
              return const Center(child: Text('Hozircha restoran yo\'q'));
            }
            return ListView.builder(
              itemCount: list.length,
              itemBuilder: (context, i) {
                final r = list[i] as Map<String, dynamic>;
                final open = r['open'] == true;
                return ListTile(
                  leading: const CircleAvatar(child: Icon(Icons.storefront)),
                  title: Text(r['name'] ?? ''),
                  subtitle: Text(r['address'] ?? ''),
                  trailing: open
                      ? const Text('Ochiq',
                          style: TextStyle(color: Colors.green))
                      : const Text('Yopiq',
                          style: TextStyle(color: Colors.red)),
                  enabled: open,
                  onTap: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => MenuScreen(
                        restaurantId: r['id'],
                        restaurantName: r['name'] ?? '',
                      ),
                    ),
                  ),
                );
              },
            );
          },
        ),
      ),
    );
  }
}
