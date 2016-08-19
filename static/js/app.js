( function () {

	window.App = {
		Views: {},
		Router: {},
		Utilities: {}
	};

	var seqId = 1;
	window.callbacks = {};
	window.UsersOnline = {};

	App.Utilities.ShowError = function ( msg ) {
		$( '#error_text' ).html( msg ).show();
		setTimeout( function () {
			$( '#error_text' ).hide();
		}, 2000 );
	};

	App.Utilities.ShowNotification = function ( msg ) {
		$( '#notification' ).html( msg ).show();
		setTimeout( function () {
			$( '#notification' ).hide();
		}, 5000 );
	};

	App.Utilities.redrawFriendsRequestCount = function () {
		if ( friendsRequestsCount > 0 ) {
			$('#friends_badge').html( friendsRequestsCount ).show();
		} else {
			$('#friends_badge').hide();
		}
	};

	App.Utilities.redrawUsersOnline = function () {
		var element = $( '#online-data' );
		var tmp = "";
		_.each( UsersOnline, function ( item ) {
			tmp += item + "<br>"
		} );
		
		element.html( tmp );
	}; 

	var wSocket = {
		ws: null,

		init: function () {
			ws = new WebSocket( 'ws://' + window.location.host + "/events" );
			
			ws.onopen = function () {
				console.log( 'Connection open' );

				$( '#status' ).html( '<span style="color: green";>•</span>' );

				new App.Router();
				Backbone.history.start();
			};

			ws.onclose = function () {
				console.log( 'Connection close' );

				$( '#status' ).html( '<span style="color: red";>•</span>' );
			};

			ws.onmessage = function ( e ) {
				var reply = JSON.parse( e.data );

				switch ( reply.Type ) {
					case "EVENT_ONLINE_USERS_LIST":
						_.each( reply.Users, function ( item ) {
							if ( item.Id != ourUserId ) {
								if ( UsersOnline[item.Id] == undefined ) {
									UsersOnline[item.Id] = item.Name;
								}
							}
						} );
						App.Utilities.redrawUsersOnline();
					break;
					case "EVENT_USER_CONNECTED":
						if ( reply.Id != ourUserId ) {
							if ( UsersOnline[reply.Id] == undefined ) {
								UsersOnline[reply.Id] = reply.Name;
							}
						}
						App.Utilities.redrawUsersOnline();
					break;
					case "EVENT_USER_DISCONNECTED":
						delete UsersOnline[reply.Id];
						App.Utilities.redrawUsersOnline();
					break;
					case "EVENT_NEW_MESSAGE":
						var url = window.location.hash.split( '/' );
						if ( ( url[1] == 'messages' ) && ( url[2] == reply.UserFrom) ) {
							App.Views.MessagesView.messages.push( reply );
							App.Views.MessagesView.render();
						} else {
							App.Utilities.ShowNotification( 'New message: ' + reply.Text );
						}
					break;
					case "EVENT_NEW_TIMELINE_EVENT":
						App.Views.TimelineView.data.unshift( reply );
						if ( window.location.hash == '' || window.location.hash == "#/" ) {
							App.Views.TimelineView.render();
						}
					break;
					case "EVENT_FRIEND_REQUEST":
						console.log( reply );
						friendsRequestsCount++;
						App.Utilities.redrawFriendsRequestCount();
					break;
					default:
						if ( reply.Type == 'REPLY_ERROR' ) {
							console.log( "Error: " + reply.Message );
							App.Utilities.ShowError( reply.Message );
						} else {
							window.callbacks[reply.SeqId]( reply );
						}
					break;
				}
			};

			this.ws = ws;
		},

		send: function ( reqType, reqData, callback ) {
			var cb = function () {
				var msg = reqType + " " + seqId + "\n" + JSON.stringify( reqData );
				this.ws.send( msg );
				window.callbacks[seqId] = callback;
				seqId++;
			}

			cb();
		}
	};

	App.Router = Backbone.Router.extend( {
		routes: {
			'': 'TimelinePage',
			'profile/:id': 'ProfilePage',
			'messages': 'MessagesPage',
			'messages/:id': 'FullMessagesPage',
			'friends': 'FriendsPage',
			'users': 'UsersPage',
			'settings': 'SettingsPage'
		},

		TimelinePage: function () {
			App.Views.TimelineView = new TimelineView();

			wSocket.send( "REQUEST_GET_TIMELINE", { Limit: 10 }, function ( data ) {
				App.Views.TimelineView.data = data.Messages;
				App.Views.TimelineView.render();
			} );
		},

		ProfilePage: function ( id ) {
			App.Views.ProfileView = new ProfileView();

			wSocket.send( "REQUEST_GET_PROFILE", { UserId: id }, function ( data ) {
				App.Views.ProfileView.render( data );
			} );
		},

		MessagesPage: function () {
			App.Views.MessagesView = new MessagesView();

			wSocket.send( 'REQUEST_GET_MESSAGES_USERS', { Limit: 10 }, function ( users ) {
				App.Views.MessagesView.users = users.Users;
				App.Views.MessagesView.render();
			} );
		},

		FullMessagesPage: function ( id ) {
			if ( App.Views.MessagesView == undefined ) {
				App.Views.MessagesView = new MessagesView();
			}

			App.Views.MessagesView.user_id = id;

			wSocket.send( 'REQUEST_GET_MESSAGES_USERS', { Limit: 10 }, function ( users ) {
				wSocket.send( 'REQUEST_GET_MESSAGES', { UserTo: id, Limit: 10 }, function ( messages ) {
					App.Views.MessagesView.global_loader = true;
					App.Views.MessagesView.users = users.Users;
					App.Views.MessagesView.messages = messages.Messages.reverse();
					App.Views.MessagesView.render();
				} );
			} );
		},

		FriendsPage: function () {
			App.Views.FriendsView = new FriendsView();
			wSocket.send( 'REQUEST_GET_FRIENDS', { Limit: 50 }, function ( data ) {
				App.Views.FriendsView.render( data );
			} );
		},

		UsersPage: function () {
			App.Views.UsersView = new UsersView();
			wSocket.send( 'REQUEST_GET_USERS_LIST', { Limit: 50 }, function ( data ) {
				App.Views.UsersView.render(  data.Users );
			});
		},

		SettingsPage: function () {
			wSocket.send( "REQUEST_GET_PROFILE", { UserId: ourUserId }, function ( data ) {
				App.Views.SettingsView = new SettingsView();
				App.Views.SettingsView.render( data );
			});
		}
	} );

	var TimelineView = Backbone.View.extend( {
		el: $( '#content' ),

		data: [],

		global_loader: true,

		events: {
            'click #send_timeline_msg': 'AddNewMsgBtn',
            'click #timeline_show_more': 'LoadMessages'
        },

		template: _.template( $( '#TimelineTemplate' ).html() ),

		AddNewMsgBtn: function () {
			var text = $( '#timeline_msg' ).val();
			wSocket.send( 'REQUEST_ADD_TO_TIMELINE', { Text: text }, function ( data ) {});
		},

		LoadMessages: function () {
			var ts = this.data[ this.data.length - 1 ].Ts;

			wSocket.send( 'REQUEST_GET_TIMELINE', { DateEnd: ts, Limit: 11, }, function ( data ) {
				if ( data.Messages.length < 11 ) {
					App.Views.TimelineView.global_loader = false;
				}

				App.Views.TimelineView.data = App.Views.TimelineView.data.concat( data.Messages );
				App.Views.TimelineView.render();
			});
		},

		render: function () {

			var triger = false;
			if ( ( ( this.data.length % 10 == 0 ) || ( this.data.length > 10 ) ) && this.global_loader ) {
				triger = true;
			}

			$( this.el ).html( this.template( { messages: this.data, loader: triger } ) );
		}
	} );

	var ProfileView = Backbone.View.extend( {
		el: $( '#content' ),

		template: _.template( $( '#ProfileTemplate' ).html() ),

		render: function ( profile ) {
			var SEX_TYPES = { 1: 'Male', 2: 'Female' };
			var FAMILY_POSITION_TYPE = { 1: 'Single', 2: 'Married' };

			profile.Sex = SEX_TYPES[profile.Sex];
			profile.FamilyPosition = FAMILY_POSITION_TYPE[profile.FamilyPosition];

			$( this.el ).html( this.template( { profile: profile } ) );
		}
	} );

	var MessagesView = Backbone.View.extend( {
		el: $( '#content' ),

		user_id: 0,
		global_loader: true,

		messages: [],
		users: [],

		template: _.template( $( '#MessagesTemplate' ).html() ),

		events: {
			'click #send_msg': 'SendMessage',
			'click #messages_show_more': 'LoadMessages'
		},

		SendMessage: function () {
			if ( App.Views.MessagesView.user_id != 0 ) {
				var text = $( '#msg' ).val();
				wSocket.send( 'REQUEST_SEND_MESSAGE', {UserTo: App.Views.MessagesView.user_id, Text: text}, function ( data ) {} );
			}
		},

		LoadMessages: function () {
			var ts = this.messages[0].Ts;
			wSocket.send( 'REQUEST_GET_MESSAGES', { UserTo: this.user_id, DateEnd: ts, Limit: 11 }, function ( data ) {
				if ( data.Messages.length < 11 ) {
					App.Views.MessagesView.global_loader = false;
				}
				App.Views.MessagesView.messages = data.Messages.concat( App.Views.MessagesView.messages );
				App.Views.MessagesView.render();
			} );
		},

		render: function () {
			var triger = false;
			
			if ( ( ( this.messages.length % 10 == 0 ) || ( this.messages.length > 10 ) ) && this.global_loader ) {
				triger = true;
			}

			if ( this.messages.length == 0 ) {
				triger = false;
			}
		
			$( this.el ).html( this.template( { users: this.users, messages: this.messages, loader: triger } ) );
		}
	} );

	var FriendsView = Backbone.View.extend( {
		el: $( '#content' ),

		events: {
			'click .request-friends': 'RequestAdd'
		},

		template: _.template( $( '#FriendsTemplate' ).html() ),

		RequestAdd: function ( ev ) {
			var id = $( ev.currentTarget ).attr( 'data-id' );
			wSocket.send( 'REQUEST_CONFIRM_FRIENDSHIP', { FriendId: id }, function ( data ) {
				$( ev.currentTarget ).text( 'friendship confirmed' );
				friendsRequestsCount--;
				App.Utilities.redrawFriendsRequestCount();
			} );
		},

		render: function ( data ) {
			$( this.el ).html( this.template( { friends: data.Users, friends_request: data.FriendRequests } ) );
		}
	} );

	var UsersView = Backbone.View.extend( {
		el: $( '#content' ),

		events: {
			'click .add-friends': 'AddFriends'
		},

		template: _.template( $( '#UsersTemplate' ).html() ),

		AddFriends: function ( ev ) {
			var id = $( ev.currentTarget ).attr( 'data-id' );
			wSocket.send( 'REQUEST_ADD_FRIEND', {FriendId: id}, function ( data ) {
				$( ev.currentTarget ).text( 'request has been sent' );
			} );
		},

		render: function ( data ) {
			$( this.el ).html( this.template( { users: data } ) );
		}
	} );

	var SettingsView = Backbone.View.extend( {
		el: $( '#content' ),

		events: {
			'click #profile-save': 'SaveProfile'
		},

		template: _.template( $( '#SettingsTemplate' ).html() ),

		SaveProfile: function () {
			var
				Name = $( "#Name" ).val(),
				Birthdate = $( '#Birthdate' ).val(),
				Sex = parseInt( $( '#Sex' ).val() ),
				City = $( '#City' ).val(),
				Position = parseInt( $( '#Position' ).val() );

			if ( Name && Birthdate && Sex && City && Position ) {
				wSocket.send( 'REQUEST_UPDATE_PROFILE', { Name: Name, Birthdate: Birthdate, Sex: Sex, CityName: City, FamilyPosition: Position }, function ( data ) {
					App.Utilities.ShowNotification( 'Profile updated' );
				} );
			} else {
				App.Utilities.ShowError( 'Fields is empty' );
			}
		},

		render: function ( data ) {
			$( this.el ).html( this.template( data ) );
		}
	} );

	wSocket.init();
	App.Utilities.redrawFriendsRequestCount();

} )();